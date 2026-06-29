package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aioproxy/aioproxy/internal/config"
	"github.com/aioproxy/aioproxy/internal/core"
	"github.com/aioproxy/aioproxy/internal/storage"
)

func TestSingBoxRefreshUsesPluginValidationConcurrencyCap(t *testing.T) {
	cfg := &config.Config{}
	cfg.ApplyDefaults()
	cfg.Validation.URL = "http://validation.test/"
	cfg.Validation.Timeout = config.Duration{Duration: 2 * time.Second}
	cfg.Validation.Concurrency = 50
	cfg.Plugins.SingBox = &config.SingBoxConfig{RefreshInterval: config.Duration{Duration: time.Hour}, ValidationConcurrency: 3, Sources: []config.SingBoxSourceConfig{{Name: "fake", Type: "inline", URL: "outbounds: []"}}}
	store := storage.New(t.TempDir(), 2)
	pool := core.NewPool()
	pm := &pluginManager{cfg: cfg, pool: pool, store: store, logger: slog.New(slog.NewTextHandler(io.Discard, nil)), status: map[string]core.PluginStatus{}}
	fp := &fakeValidationPlugin{name: "singbox", count: 20, source: "fake"}

	pm.refresh(context.Background(), fp)

	if max := fp.dialer.max.Load(); max > int32(cfg.Plugins.SingBox.ValidationConcurrency) {
		t.Fatalf("max concurrent singbox validations=%d, cap=%d", max, cfg.Plugins.SingBox.ValidationConcurrency)
	}
	total, available := pool.Count()
	if total != fp.count || available != fp.count {
		t.Fatalf("pool total=%d available=%d want=%d", total, available, fp.count)
	}
}

func TestConfigPathRejectsUnknownOrExtraArgs(t *testing.T) {
	if path, ok := configPath([]string{"-c", "config.yaml"}); !ok || path != "config.yaml" {
		t.Fatalf("valid config path parse path=%q ok=%v", path, ok)
	}
	for _, args := range [][]string{
		{"-c"},
		{"--unknown", "x", "-c", "config.yaml"},
		{"-c", "config.yaml", "extra"},
	} {
		if path, ok := configPath(args); ok {
			t.Fatalf("configPath(%v)=%q,true; want rejection", args, path)
		}
	}
}

type fakeValidationPlugin struct {
	name   string
	count  int
	source string
	dialer cappedValidationDialer
}

func (p *fakeValidationPlugin) Name() string                   { return p.name }
func (p *fakeValidationPlugin) Active() bool                   { return true }
func (p *fakeValidationPlugin) RefreshInterval() time.Duration { return time.Hour }
func (p *fakeValidationPlugin) Refresh(context.Context) core.PluginResult {
	candidates := make([]core.Candidate, 0, p.count)
	dialers := make(map[string]core.CandidateDialer, p.count)
	for i := 0; i < p.count; i++ {
		c := core.Candidate{Protocol: core.ProtocolSingBox, Source: p.name, Name: fmt.Sprintf("node-%d", i), Metadata: map[string]string{"source": p.source, "tag": fmt.Sprintf("node-%d", i), "config_hash": fmt.Sprintf("hash-%d", i)}}
		c.Normalize()
		candidates = append(candidates, c)
		dialers[c.Fingerprint] = &p.dialer
	}
	return core.PluginResult{
		Candidates: candidates,
		Dialers:    dialers,
		Reports:    []core.ImportReport{{Plugin: p.name, Source: p.source, StartedAt: time.Now(), FinishedAt: time.Now(), Total: p.count, Imported: p.count}},
	}
}

type cappedValidationDialer struct {
	active atomic.Int32
	max    atomic.Int32
}

func (d *cappedValidationDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	active := d.active.Add(1)
	defer d.active.Add(-1)
	for {
		max := d.max.Load()
		if active <= max || d.max.CompareAndSwap(max, active) {
			break
		}
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(25 * time.Millisecond):
	}
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		req, err := http.ReadRequest(bufio.NewReader(server))
		if err != nil {
			return
		}
		_ = req.Body.Close()
		_, _ = io.WriteString(server, "HTTP/1.1 204 No Content\r\nContent-Length: 0\r\n\r\n")
	}()
	return client, nil
}
