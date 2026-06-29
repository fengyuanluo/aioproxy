package admin

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
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
			if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.Token)) != 1 {
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
		ID, Fingerprint, Protocol, Host, Source, Name, Status, LastError, CountryCode, Country string
		Port, FailureCount                                                                     int
		LastValidation                                                                         time.Time
	}
	q := r.URL.Query()
	limit := boundedInt(q.Get("limit"), 1000, 1, 5000)
	offset := boundedInt(q.Get("offset"), 0, 0, 1<<30)
	source := strings.TrimSpace(q.Get("source"))
	status := strings.TrimSpace(q.Get("status"))
	protocol := strings.TrimSpace(q.Get("protocol"))
	totalMatched := 0
	returned := 0
	out := make([]view, 0, min(limit, 256))
	s.pool.ForEach(func(c core.Candidate) bool {
		if source != "" && !strings.EqualFold(c.Source, source) {
			return true
		}
		if status != "" && !strings.EqualFold(c.Status, status) {
			return true
		}
		if protocol != "" && !strings.EqualFold(c.Protocol, protocol) {
			return true
		}
		totalMatched++
		if totalMatched <= offset {
			return true
		}
		if returned >= limit {
			return true
		}
		out = append(out, view{c.ID, c.Fingerprint[:16], c.Protocol, c.Host, c.Source, c.Name, c.Status, c.LastError, c.Metadata["country_code"], c.Metadata["country"], c.Port, c.FailureCount, c.LastValidation})
		returned++
		return true
	})
	w.Header().Set("X-Pool-Matched", strconv.Itoa(totalMatched))
	w.Header().Set("X-Pool-Returned", strconv.Itoa(returned))
	w.Header().Set("X-Pool-Limit", strconv.Itoa(limit))
	w.Header().Set("X-Pool-Offset", strconv.Itoa(offset))
	writeJSON(w, out)
}
func (s *Server) plugins(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, safePluginStatuses(s.statuses()))
}
func (s *Server) snapshots(w http.ResponseWriter, r *http.Request) {
	files, _ := s.store.SnapshotFiles()
	writeJSON(w, map[string]any{"files": files})
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func safePluginStatuses(statuses []core.PluginStatus) []core.PluginStatus {
	out := make([]core.PluginStatus, len(statuses))
	copy(out, statuses)
	for i := range out {
		reports := make([]core.ImportReport, len(out[i].Reports))
		copy(reports, out[i].Reports)
		for j := range reports {
			reports[j].Metadata = safeReportMetadata(reports[j].Metadata)
		}
		out[i].Reports = reports
	}
	return out
}

func safeReportMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	allowed := map[string]struct{}{"type": {}, "protocol": {}}
	out := map[string]string{}
	for k, v := range in {
		if _, ok := allowed[k]; ok {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func boundedInt(raw string, def, minVal, maxVal int) int {
	if strings.TrimSpace(raw) == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if v < minVal {
		return minVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
