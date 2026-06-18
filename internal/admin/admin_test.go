package admin

import (
	"io"
	"log/slog"
	"net"
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
