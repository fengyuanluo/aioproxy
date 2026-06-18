package singbox

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	S "github.com/sagernet/sing/service"
	"gopkg.in/yaml.v3"

	"github.com/aioproxy/aioproxy/internal/config"
	"github.com/aioproxy/aioproxy/internal/core"
)

type Plugin struct {
	cfg    config.SingBoxConfig
	client *http.Client
	mu     sync.Mutex
	boxes  []*box.Box
}

func New(cfg config.SingBoxConfig) *Plugin {
	return &Plugin{cfg: cfg, client: &http.Client{Timeout: 60 * time.Second}}
}
func (p *Plugin) Name() string                   { return "singbox" }
func (p *Plugin) Active() bool                   { return len(p.cfg.Sources) > 0 }
func (p *Plugin) RefreshInterval() time.Duration { return p.cfg.RefreshInterval.Duration }

func (p *Plugin) Refresh(ctx context.Context) core.PluginResult {
	p.closeBoxes()
	var all []core.Candidate
	dialers := map[string]core.CandidateDialer{}
	var reports []core.ImportReport
	var newBoxes []*box.Box
	for _, src := range p.cfg.Sources {
		started := time.Now()
		rep := core.ImportReport{Plugin: p.Name(), Source: src.Name, StartedAt: started, SkipReasons: map[string]int{}, Metadata: map[string]string{"type": src.Type}}
		content, err := p.readSource(ctx, src)
		if err != nil {
			rep.Error = err.Error()
			rep.FinishedAt = time.Now()
			reports = append(reports, rep)
			continue
		}
		cands, ds, b, err := p.importContent(ctx, src.Name, content, &rep)
		if err != nil {
			rep.Error = err.Error()
		}
		if b != nil {
			newBoxes = append(newBoxes, b)
		}
		rep.Imported = len(cands)
		rep.FinishedAt = time.Now()
		reports = append(reports, rep)
		all = append(all, cands...)
		for k, v := range ds {
			dialers[k] = v
		}
	}
	p.mu.Lock()
	p.boxes = newBoxes
	p.mu.Unlock()
	return core.PluginResult{Candidates: all, Dialers: dialers, Reports: reports}
}

func (p *Plugin) Close() { p.closeBoxes() }
func (p *Plugin) closeBoxes() {
	p.mu.Lock()
	boxes := p.boxes
	p.boxes = nil
	p.mu.Unlock()
	for _, b := range boxes {
		_ = b.Close()
	}
}

func (p *Plugin) readSource(ctx context.Context, src config.SingBoxSourceConfig) ([]byte, error) {
	switch src.Type {
	case "url":
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("source status %s", resp.Status)
		}
		return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	case "file":
		return os.ReadFile(src.Path)
	case "inline":
		return []byte(src.URL), nil
	default:
		return nil, fmt.Errorf("unsupported source type %s", src.Type)
	}
}

func (p *Plugin) importContent(ctx context.Context, source string, content []byte, rep *core.ImportReport) ([]core.Candidate, map[string]core.CandidateDialer, *box.Box, error) {
	outbounds, err := parseToOutbounds(content, rep)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(outbounds) == 0 {
		return nil, nil, nil, fmt.Errorf("zero convertible sing-box outbounds")
	}
	cfgMap := map[string]any{"log": map[string]any{"disabled": true}, "outbounds": outbounds, "route": map[string]any{"final": outbounds[0]["tag"]}}
	jsonBytes, _ := json.Marshal(cfgMap)
	ctx = include.Context(ctx)
	var opts option.Options
	if err := opts.UnmarshalJSONContext(ctx, jsonBytes); err != nil {
		return nil, nil, nil, err
	}
	b, err := box.New(box.Options{Options: opts, Context: ctx})
	if err != nil {
		return nil, nil, nil, err
	}
	if err := b.Start(); err != nil {
		_ = b.Close()
		return nil, nil, nil, err
	}
	var cands []core.Candidate
	dialers := map[string]core.CandidateDialer{}
	configHash := shortHash(jsonBytes)
	for _, ob := range outbounds {
		tag, _ := ob["tag"].(string)
		typ, _ := ob["type"].(string)
		if tag == "" {
			continue
		}
		outbound, ok := b.Outbound().Outbound(tag)
		if !ok {
			rep.AddSkip("outbound_not_found")
			continue
		}
		server := fmt.Sprint(ob["server"])
		port := 0
		switch v := ob["server_port"].(type) {
		case int:
			port = v
		case float64:
			port = int(v)
		case uint16:
			port = int(v)
		}
		c := core.Candidate{Protocol: core.ProtocolSingBox, Host: server, Port: port, Source: "singbox", Name: tag, Metadata: map[string]string{"tag": tag, "type": typ, "config_hash": configHash}}
		c.Normalize()
		cands = append(cands, c)
		dialers[c.Fingerprint] = &outboundDialer{outbound: outbound}
	}
	return cands, dialers, b, nil
}

type outboundDialer struct{ outbound adapter.Outbound }

func (d *outboundDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.outbound.DialContext(ctx, network, M.ParseSocksaddr(address))
}
func (d *outboundDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return d.outbound.ListenPacket(ctx, destination)
}

func parseToOutbounds(content []byte, rep *core.ImportReport) ([]map[string]any, error) {
	trim := strings.TrimSpace(string(content))
	if trim == "" {
		return nil, fmt.Errorf("empty content")
	}
	// sing-box native JSON/YAML with outbounds.
	var top map[string]any
	if err := yaml.Unmarshal(content, &top); err == nil {
		if raw, ok := top["outbounds"]; ok {
			return normalizeOutboundSlice(raw, rep), nil
		}
		if raw, ok := top["proxies"]; ok {
			return clashProxiesToOutbounds(raw, rep), nil
		}
	}
	// base64 share-link list.
	if decoded, err := base64.StdEncoding.DecodeString(stripWhitespace(trim)); err == nil && strings.Contains(string(decoded), "://") {
		trim = string(decoded)
	}
	var out []map[string]any
	for _, line := range strings.Fields(trim) {
		if strings.Contains(line, "://") {
			rep.Total++
			if ob, err := shareLinkToOutbound(line, fmt.Sprintf("node-%d", rep.Total)); err == nil {
				out = append(out, ob)
			} else {
				rep.AddSkip("share_" + err.Error())
			}
		}
	}
	return out, nil
}

func normalizeOutboundSlice(raw any, rep *core.ImportReport) []map[string]any {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for i, item := range arr {
		rep.Total++
		m, ok := toMap(item)
		if !ok {
			rep.AddSkip("invalid_outbound")
			continue
		}
		if _, ok := m["tag"]; !ok {
			m["tag"] = fmt.Sprintf("outbound-%d", i)
		}
		if convertibleType(fmt.Sprint(m["type"])) {
			out = append(out, m)
		} else {
			rep.AddSkip("unsupported_" + fmt.Sprint(m["type"]))
		}
	}
	return out
}

func clashProxiesToOutbounds(raw any, rep *core.ImportReport) []map[string]any {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	var out []map[string]any
	for i, item := range arr {
		rep.Total++
		m, ok := toMap(item)
		if !ok {
			rep.AddSkip("invalid_proxy")
			continue
		}
		ob, err := clashProxyToOutbound(m, i)
		if err != nil {
			rep.AddSkip(err.Error())
			continue
		}
		out = append(out, ob)
	}
	return out
}

func clashProxyToOutbound(m map[string]any, i int) (map[string]any, error) {
	typ := strings.ToLower(fmt.Sprint(m["type"]))
	tag := fmt.Sprint(m["name"])
	if tag == "" || tag == "<nil>" {
		tag = fmt.Sprintf("clash-%d", i)
	}
	server := fmt.Sprint(m["server"])
	port := toInt(m["port"])
	if server == "" || server == "<nil>" || port == 0 {
		return nil, fmt.Errorf("missing_server_or_port")
	}
	ob := map[string]any{"type": typ, "tag": tag, "server": server, "server_port": port}
	switch typ {
	case "ss":
		ob["type"] = "shadowsocks"
		ob["method"] = fmt.Sprint(m["cipher"])
		ob["password"] = fmt.Sprint(m["password"])
	case "socks5":
		ob["type"] = "socks"
		ob["version"] = "5"
		copyStr(ob, m, "username", "username")
		copyStr(ob, m, "password", "password")
	case "http":
		copyStr(ob, m, "username", "username")
		copyStr(ob, m, "password", "password")
	case "vmess":
		ob["uuid"] = fmt.Sprint(m["uuid"])
		ob["security"] = defaultStr(fmt.Sprint(m["cipher"]), "auto")
		applyV2RayClash(ob, m)
	case "vless":
		ob["uuid"] = fmt.Sprint(m["uuid"])
		copyStr(ob, m, "flow", "flow")
		applyV2RayClash(ob, m)
	case "trojan":
		ob["password"] = fmt.Sprint(m["password"])
		applyTLS(ob, m, true)
		applyTransport(ob, m)
	case "hysteria2":
		ob["password"] = fmt.Sprint(first(m["password"], m["auth"]))
		applyTLS(ob, m, true)
	case "hysteria":
		ob["auth_str"] = fmt.Sprint(first(m["auth-str"], m["auth_str"], m["auth"]))
		applyTLS(ob, m, true)
	default:
		return nil, fmt.Errorf("unsupported_%s", typ)
	}
	return ob, nil
}

func shareLinkToOutbound(raw, fallbackTag string) (map[string]any, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid")
	}
	scheme := strings.ToLower(u.Scheme)
	tag := u.Fragment
	if tag == "" {
		tag = fallbackTag
	}
	switch scheme {
	case "ss":
		return parseSSLink(u, tag)
	case "socks", "socks5":
		port, _ := strconv.Atoi(u.Port())
		ob := map[string]any{"type": "socks", "tag": tag, "server": u.Hostname(), "server_port": port, "version": "5"}
		if u.User != nil {
			ob["username"] = u.User.Username()
			pw, _ := u.User.Password()
			ob["password"] = pw
		}
		return ob, nil
	case "vmess":
		return parseVMessLink(raw, tag)
	case "vless":
		port, _ := strconv.Atoi(u.Port())
		ob := map[string]any{"type": "vless", "tag": tag, "server": u.Hostname(), "server_port": port, "uuid": u.User.Username()}
		q := u.Query()
		if flow := q.Get("flow"); flow != "" {
			ob["flow"] = flow
		}
		applyV2RayQuery(ob, q)
		return ob, nil
	case "trojan":
		port, _ := strconv.Atoi(u.Port())
		ob := map[string]any{"type": "trojan", "tag": tag, "server": u.Hostname(), "server_port": port, "password": u.User.Username()}
		q := u.Query()
		applyTLSQuery(ob, q, true)
		applyTransportQuery(ob, q)
		return ob, nil
	case "hysteria2", "hy2":
		port, _ := strconv.Atoi(u.Port())
		ob := map[string]any{"type": "hysteria2", "tag": tag, "server": u.Hostname(), "server_port": port, "password": u.User.Username()}
		applyTLSQuery(ob, u.Query(), true)
		return ob, nil
	case "hysteria":
		port, _ := strconv.Atoi(u.Port())
		ob := map[string]any{"type": "hysteria", "tag": tag, "server": u.Hostname(), "server_port": port, "auth_str": u.User.Username()}
		applyTLSQuery(ob, u.Query(), true)
		return ob, nil
	default:
		return nil, fmt.Errorf("unsupported_%s", scheme)
	}
}

func parseSSLink(u *url.URL, tag string) (map[string]any, error) {
	server := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	user := u.User.Username()
	pw, _ := u.User.Password()
	if !strings.Contains(user, ":") {
		if dec, err := base64.RawURLEncoding.DecodeString(user); err == nil {
			user = string(dec)
		} else if dec, err := base64.StdEncoding.DecodeString(user); err == nil {
			user = string(dec)
		}
	}
	if strings.Contains(user, ":") && pw == "" {
		parts := strings.SplitN(user, ":", 2)
		user = parts[0]
		pw = parts[1]
	}
	if server == "" || port == 0 || user == "" {
		return nil, fmt.Errorf("invalid_ss")
	}
	return map[string]any{"type": "shadowsocks", "tag": tag, "server": server, "server_port": port, "method": user, "password": pw}, nil
}

func parseVMessLink(raw, tag string) (map[string]any, error) {
	body := strings.TrimPrefix(raw, "vmess://")
	if i := strings.IndexByte(body, '#'); i >= 0 {
		body = body[:i]
	}
	dec, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		dec, err = base64.RawStdEncoding.DecodeString(body)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid_vmess")
	}
	var m map[string]any
	if err := json.Unmarshal(dec, &m); err != nil {
		return nil, fmt.Errorf("invalid_vmess_json")
	}
	port := toInt(m["port"])
	ob := map[string]any{"type": "vmess", "tag": defaultStr(fmt.Sprint(m["ps"]), tag), "server": fmt.Sprint(m["add"]), "server_port": port, "uuid": fmt.Sprint(m["id"]), "security": defaultStr(fmt.Sprint(m["scy"]), "auto")}
	if netw := fmt.Sprint(m["net"]); netw != "" && netw != "tcp" {
		q := url.Values{}
		q.Set("type", netw)
		q.Set("path", fmt.Sprint(m["path"]))
		q.Set("host", fmt.Sprint(m["host"]))
		applyTransportQuery(ob, q)
	}
	if tlsv := fmt.Sprint(m["tls"]); tlsv == "tls" {
		ob["tls"] = map[string]any{"enabled": true, "server_name": fmt.Sprint(m["sni"]), "insecure": toBool(m["skip-cert-verify"])}
	}
	return ob, nil
}

func applyV2RayClash(ob map[string]any, m map[string]any) {
	applyTLS(ob, m, false)
	applyTransport(ob, m)
}
func applyTLS(ob map[string]any, m map[string]any, def bool) {
	enabled := def || toBool(first(m["tls"], m["skip-cert-verify"]))
	if s := fmt.Sprint(m["sni"]); enabled || s != "" {
		ob["tls"] = map[string]any{"enabled": enabled, "server_name": s, "insecure": toBool(m["skip-cert-verify"])}
	}
}
func applyTransport(ob map[string]any, m map[string]any) {
	q := url.Values{}
	for _, k := range []string{"network", "type", "ws-opts", "grpc-opts"} {
		_ = k
	}
	netw := fmt.Sprint(first(m["network"], m["net"]))
	if netw == "" || netw == "tcp" {
		return
	}
	q.Set("type", netw)
	if path := fmt.Sprint(first(m["ws-path"], m["path"])); path != "" && path != "<nil>" {
		q.Set("path", path)
	}
	applyTransportQuery(ob, q)
}
func applyV2RayQuery(ob map[string]any, q url.Values) {
	applyTLSQuery(ob, q, false)
	applyTransportQuery(ob, q)
}
func applyTLSQuery(ob map[string]any, q url.Values, def bool) {
	sec := q.Get("security")
	enabled := def || sec == "tls" || sec == "reality"
	if enabled || q.Get("sni") != "" {
		tls := map[string]any{"enabled": enabled, "server_name": firstStr(q.Get("sni"), q.Get("host")), "insecure": q.Get("allowInsecure") == "1" || q.Get("skip-cert-verify") == "true"}
		ob["tls"] = tls
	}
}
func applyTransportQuery(ob map[string]any, q url.Values) {
	typ := firstStr(q.Get("type"), q.Get("network"))
	if typ == "" || typ == "tcp" {
		return
	}
	tr := map[string]any{"type": mapTransport(typ)}
	if p := q.Get("path"); p != "" {
		tr["path"] = p
	}
	if h := q.Get("host"); h != "" {
		tr["headers"] = map[string]any{"Host": []string{h}}
	}
	if s := q.Get("serviceName"); s != "" {
		tr["service_name"] = s
	}
	ob["transport"] = tr
}
func mapTransport(s string) string {
	switch strings.ToLower(s) {
	case "ws", "websocket":
		return "ws"
	case "grpc":
		return "grpc"
	case "httpupgrade":
		return "httpupgrade"
	default:
		return strings.ToLower(s)
	}
}

func convertibleType(t string) bool {
	switch strings.ToLower(t) {
	case "direct", "block", "socks", "http", "shadowsocks", "vmess", "vless", "trojan", "hysteria", "hysteria2", "anytls", "shadowtls", "ssh", "tuic", "naive":
		return true
	}
	return false
}
func toMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	if ok {
		return m, true
	}
	m2, ok := v.(map[any]any)
	if !ok {
		return nil, false
	}
	out := map[string]any{}
	for k, v := range m2 {
		out[fmt.Sprint(k)] = v
	}
	return out, true
}
func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case uint16:
		return int(x)
	case float64:
		return int(x)
	case string:
		i, _ := strconv.Atoi(x)
		return i
	}
	return 0
}
func toBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "1"
	}
	return false
}
func copyStr(dst map[string]any, src map[string]any, dk, sk string) {
	if v := fmt.Sprint(src[sk]); v != "" && v != "<nil>" {
		dst[dk] = v
	}
}
func defaultStr(s, d string) string {
	if s == "" || s == "<nil>" {
		return d
	}
	return s
}
func first(vals ...any) any {
	for _, v := range vals {
		if fmt.Sprint(v) != "" && fmt.Sprint(v) != "<nil>" {
			return v
		}
	}
	return ""
}
func firstStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
func stripWhitespace(s string) string { return strings.Join(strings.Fields(s), "") }
func shortHash(b []byte) string       { h := sha256.Sum256(b); return hex.EncodeToString(h[:])[:16] }

// Keep imported service package reachable for sing-box service.Context generic init in older toolchains.
var _ = S.ContextWithDefaultRegistry
