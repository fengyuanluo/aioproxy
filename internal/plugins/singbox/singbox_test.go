package singbox

import (
	"context"
	"testing"

	"github.com/aioproxy/aioproxy/internal/config"
	"github.com/aioproxy/aioproxy/internal/core"
)

func TestParseClashProxies(t *testing.T) {
	data := []byte("proxies:\n  - name: h1\n    type: http\n    server: 127.0.0.1\n    port: 8080\n  - name: s1\n    type: socks5\n    server: 127.0.0.1\n    port: 1080\n  - name: bad\n    type: mieru\n    server: x\n    port: 1\n")
	rep := &core.ImportReport{SkipReasons: map[string]int{}}
	obs, err := parseToOutbounds(data, rep)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 2 {
		t.Fatalf("outbounds=%d skips=%v", len(obs), rep.SkipReasons)
	}
}
func TestParseBase64ShareList(t *testing.T) {
	data := []byte("c3M6Ly9ZV1Z6TFRJMU5pMW5ZMjA2Y0dGemN3QDEuMi4zLjQ6ODM4OCNzcwo=\n")
	rep := &core.ImportReport{SkipReasons: map[string]int{}}
	obs, err := parseToOutbounds(data, rep)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 {
		t.Fatalf("outbounds=%d skips=%v", len(obs), rep.SkipReasons)
	}
}

func TestBuildSingleHTTPOutbound(t *testing.T) {
	ob := map[string]any{"type": "http", "tag": "h", "server": "127.0.0.1", "server_port": 8080}
	c, d, b, err := buildSingleOutbound(context.Background(), ob, "test")
	if err != nil {
		t.Fatal(err)
	}
	if d == nil || c.Fingerprint == "" {
		t.Fatal("missing dialer/candidate")
	}
	_ = b.Close()
}

func TestUnsupportedTUICAndNaiveAreSkippedBeforeBuild(t *testing.T) {
	data := []byte("outbounds:\n  - type: tuic\n    tag: t1\n    server: 127.0.0.1\n    server_port: 443\n  - type: naive\n    tag: n1\n    server: 127.0.0.1\n    server_port: 443\n")
	rep := &core.ImportReport{SkipReasons: map[string]int{}}
	obs, err := parseToOutbounds(data, rep)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 0 {
		t.Fatalf("outbounds=%d want=0", len(obs))
	}
	if rep.SkipReasons["unsupported_tuic"] != 1 || rep.SkipReasons["unsupported_naive"] != 1 {
		t.Fatalf("skip reasons=%v", rep.SkipReasons)
	}
}

func TestRefreshInvalidURLReturnsReportError(t *testing.T) {
	p := New(config.SingBoxConfig{Sources: []config.SingBoxSourceConfig{{Name: "bad", Type: "url", URL: "http://%zz"}}})
	res := p.Refresh(context.Background())
	if len(res.Reports) != 1 {
		t.Fatalf("reports=%d", len(res.Reports))
	}
	if res.Reports[0].Error == "" {
		t.Fatalf("expected report error, got %#v", res.Reports[0])
	}
	if len(res.Candidates) != 0 {
		t.Fatalf("candidates=%d", len(res.Candidates))
	}
}
