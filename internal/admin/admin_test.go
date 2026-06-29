package admin

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aioproxy/aioproxy/internal/config"
	"github.com/aioproxy/aioproxy/internal/core"
	"github.com/aioproxy/aioproxy/internal/storage"
)

func TestStartFailsWhenListenAddressBusy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	srv := New(
		config.AdminConfig{Listen: ln.Addr().String()},
		core.NewPool(),
		func() []core.PluginStatus { return nil },
		core.NewSessionManager(time.Minute, time.Hour),
		storage.New(t.TempDir(), 1),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	if err := srv.Start(); err == nil {
		t.Fatal("expected busy listen address to fail")
	}
}

func TestPoolViewSupportsPaginationAndFilters(t *testing.T) {
	pool := core.NewPool()
	pool.AddValidated([]core.Candidate{
		{Protocol: core.ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "fpl"},
		{Protocol: core.ProtocolSOCKS5, Host: "2.2.2.2", Port: 1080, Source: "fpl"},
		{Protocol: core.ProtocolHTTP, Host: "3.3.3.3", Port: 80, Source: "fofa"},
	}, nil)
	srv := New(
		config.AdminConfig{Listen: "127.0.0.1:0"},
		pool,
		func() []core.PluginStatus { return nil },
		core.NewSessionManager(time.Minute, time.Hour),
		storage.New(t.TempDir(), 1),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	req := httptest.NewRequest(http.MethodGet, "/pool?source=fpl&limit=1&offset=1", nil)
	rr := httptest.NewRecorder()
	srv.poolView(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("X-Pool-Matched"); got != "2" {
		t.Fatalf("X-Pool-Matched=%s want=2", got)
	}
	if got := rr.Header().Get("X-Pool-Returned"); got != "1" {
		t.Fatalf("X-Pool-Returned=%s want=1", got)
	}
	var rows []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d want=1 body=%s", len(rows), rr.Body.String())
	}
	if rows[0]["Host"] != "2.2.2.2" {
		t.Fatalf("unexpected row: %+v", rows[0])
	}
}

func TestPluginsViewScrubsReportMetadata(t *testing.T) {
	srv := New(
		config.AdminConfig{Listen: "127.0.0.1:0"},
		core.NewPool(),
		func() []core.PluginStatus {
			return []core.PluginStatus{{
				Name:   "x",
				Active: true,
				Reports: []core.ImportReport{{
					Plugin:   "x",
					Source:   "s",
					Metadata: map[string]string{"type": "inline", "protocol": "http", "raw_url": "secret", "token": "secret"},
				}},
			}}
		},
		core.NewSessionManager(time.Minute, time.Hour),
		storage.New(t.TempDir(), 1),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	req := httptest.NewRequest(http.MethodGet, "/plugins", nil)
	rr := httptest.NewRecorder()
	srv.plugins(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, "secret") || strings.Contains(body, "raw_url") || strings.Contains(body, "token") {
		t.Fatalf("plugins response leaked unsafe metadata: %s", body)
	}
	if !strings.Contains(body, "inline") || !strings.Contains(body, "http") {
		t.Fatalf("plugins response removed safe metadata: %s", body)
	}
}
