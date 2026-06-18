package admin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/aioproxy/aioproxy/internal/config"
	"github.com/aioproxy/aioproxy/internal/core"
	"github.com/aioproxy/aioproxy/internal/storage"
)

type Server struct {
	cfg      config.AdminConfig
	pool     *core.Pool
	statuses func() []core.PluginStatus
	sessions *core.SessionManager
	store    *storage.Store
	logger   *slog.Logger
	srv      *http.Server
}

func New(cfg config.AdminConfig, pool *core.Pool, statuses func() []core.PluginStatus, sessions *core.SessionManager, store *storage.Store, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, pool: pool, statuses: statuses, sessions: sessions, store: store, logger: logger}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.auth(s.health))
	mux.HandleFunc("/stats", s.auth(s.stats))
	mux.HandleFunc("/pool", s.auth(s.poolView))
	mux.HandleFunc("/plugins", s.auth(s.plugins))
	mux.HandleFunc("/snapshots", s.auth(s.snapshots))
	s.srv = &http.Server{Addr: s.cfg.Listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	ln, err := net.Listen("tcp", s.cfg.Listen)
	if err != nil {
		return err
	}
	go func() {
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Error("admin server failed", "error", err)
		}
	}()
	s.logger.Info("admin listener started", "listen", ln.Addr().String())
	return nil
}
func (s *Server) Close(timeout time.Duration) error {
	if s.srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.srv.Shutdown(ctx)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Token != "" {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token != s.cfg.Token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	total, avail := s.pool.Count()
	statuses := s.statuses()
	degraded := avail == 0 || len(statuses) == 0
	for _, st := range statuses {
		if !st.Active || st.Degraded {
			degraded = true
		}
	}
	status := "healthy"
	if degraded {
		status = "degraded"
	}
	writeJSON(w, map[string]any{"status": status, "pool_total": total, "pool_available": avail, "sessions": s.sessions.Count(), "time": time.Now()})
}
func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	total, avail := s.pool.Count()
	writeJSON(w, map[string]any{"pool_total": total, "pool_available": avail, "sessions": s.sessions.Count(), "plugins": s.statuses()})
}
func (s *Server) poolView(w http.ResponseWriter, r *http.Request) {
	type view struct {
		ID, Fingerprint, Protocol, Host, Source, Name, Status, LastError string
		Port, FailureCount                                               int
		LastValidation                                                   time.Time
	}
	var out []view
	for _, c := range s.pool.List() {
		out = append(out, view{c.ID, c.Fingerprint[:16], c.Protocol, c.Host, c.Source, c.Name, c.Status, c.LastError, c.Port, c.FailureCount, c.LastValidation})
	}
	writeJSON(w, out)
}
func (s *Server) plugins(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.statuses()) }
func (s *Server) snapshots(w http.ResponseWriter, r *http.Request) {
	files, _ := s.store.SnapshotFiles()
	writeJSON(w, map[string]any{"files": files})
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
