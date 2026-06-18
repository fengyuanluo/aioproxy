package fofa

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aioproxy/aioproxy/internal/config"
	"github.com/aioproxy/aioproxy/internal/core"
)

type Plugin struct {
	cfg    config.FOFAConfig
	client *http.Client
}

func New(cfg config.FOFAConfig) *Plugin {
	return &Plugin{cfg: cfg, client: &http.Client{Timeout: 45 * time.Second}}
}
func (p *Plugin) Name() string                   { return "fofa" }
func (p *Plugin) Active() bool                   { return strings.TrimSpace(p.cfg.Key) != "" }
func (p *Plugin) RefreshInterval() time.Duration { return p.cfg.RefreshInterval.Duration }

type searchResp struct {
	Error   bool    `json:"error"`
	Errmsg  string  `json:"errmsg"`
	Results [][]any `json:"results"`
	Size    int     `json:"size"`
}

func (p *Plugin) Refresh(ctx context.Context) core.PluginResult {
	var all []core.Candidate
	var reports []core.ImportReport
	for _, q := range p.cfg.Queries {
		started := time.Now()
		rep := core.ImportReport{Plugin: p.Name(), Source: q.Name, StartedAt: started, SkipReasons: map[string]int{}, Metadata: map[string]string{"protocol": q.Protocol}}
		cands, err := p.search(ctx, q, &rep)
		if err != nil {
			rep.Error = err.Error()
		}
		rep.Imported = len(cands)
		rep.FinishedAt = time.Now()
		reports = append(reports, rep)
		all = append(all, cands...)
	}
	return core.PluginResult{Candidates: all, Reports: reports}
}

func (p *Plugin) search(ctx context.Context, q config.FOFAQueryConfig, rep *core.ImportReport) ([]core.Candidate, error) {
	base := strings.TrimRight(p.cfg.BaseURL, "/")
	if base == "" {
		base = config.DefaultFOFABaseURL
	}
	u, err := url.Parse(base + "/api/v1/search/all")
	if err != nil {
		return nil, err
	}
	vals := u.Query()
	vals.Set("key", p.cfg.Key)
	vals.Set("qbase64", base64.StdEncoding.EncodeToString([]byte(q.Query)))
	fields := q.Fields
	if fields == "" {
		fields = config.DefaultFOFAFields
	}
	vals.Set("fields", fields)
	vals.Set("size", strconv.Itoa(p.cfg.Size))
	vals.Set("page", "1")
	u.RawQuery = vals.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fofa status %s", resp.Status)
	}
	var sr searchResp
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}
	if sr.Error {
		return nil, fmt.Errorf("fofa error: %s", sr.Errmsg)
	}
	fieldList := splitFields(fields)
	var out []core.Candidate
	for _, row := range sr.Results {
		rep.Total++
		m := map[string]string{}
		for i, f := range fieldList {
			if i < len(row) {
				m[f] = fmt.Sprint(row[i])
			}
		}
		host := firstNonEmpty(m["ip"], hostFromURL(m["host"]), m["host"])
		port, _ := strconv.Atoi(m["port"])
		if host == "" || port == 0 {
			rep.AddSkip("missing_host_or_port")
			continue
		}
		protocol := strings.ToLower(q.Protocol)
		if protocol == "sk" || protocol == "socks" {
			protocol = core.ProtocolSOCKS5
		}
		if protocol != core.ProtocolHTTP && protocol != core.ProtocolSOCKS5 {
			rep.AddSkip("unsupported_protocol")
			continue
		}
		c := core.Candidate{Protocol: protocol, Host: host, Port: port, Source: "fofa", Name: q.Name, Metadata: map[string]string{"query": q.Name}}
		c.Normalize()
		out = append(out, c)
	}
	return out, nil
}

func splitFields(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
func hostFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return ""
}
