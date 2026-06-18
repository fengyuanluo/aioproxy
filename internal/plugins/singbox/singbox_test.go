package singbox

import (
	"context"
	"testing"

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
