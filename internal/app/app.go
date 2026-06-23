package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aioproxy/aioproxy/internal/admin"
	"github.com/aioproxy/aioproxy/internal/config"
	"github.com/aioproxy/aioproxy/internal/core"
	"github.com/aioproxy/aioproxy/internal/logging"
	"github.com/aioproxy/aioproxy/internal/plugins"
	"github.com/aioproxy/aioproxy/internal/plugins/fofa"
	"github.com/aioproxy/aioproxy/internal/plugins/fpl"
	"github.com/aioproxy/aioproxy/internal/plugins/singbox"
	"github.com/aioproxy/aioproxy/internal/proxy"
	"github.com/aioproxy/aioproxy/internal/storage"
	"github.com/aioproxy/aioproxy/internal/validation"
)

func Run(parent context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "serve":
		return runServe(parent, args[1:], stdout, stderr)
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage:\n  aioproxy check -c config.yaml\n  aioproxy serve -c config.yaml")
}
func configPath(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		if args[i] == "-c" || args[i] == "--config" {
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", false
		}
	}
	return "", false
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	path, ok := configPath(args)
	if !ok {
		fmt.Fprintln(stderr, "missing -c config.yaml")
		return 2
	}
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintln(stderr, "ERROR:", err)
		return 1
	}
	res := cfg.Check()
	fmt.Fprintln(stdout, "AIOPROXY config check")
	if len(res.ActivePlugins) == 0 {
		fmt.Fprintln(stdout, "active plugins: none")
	} else {
		fmt.Fprintln(stdout, "active plugins:", strings.Join(res.ActivePlugins, ", "))
	}
	for _, e := range res.Errors {
		fmt.Fprintln(stdout, "ERROR:", e)
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(stdout, "WARNING:", w)
	}
	if !res.OK() {
		return 1
	}
	fmt.Fprintln(stdout, "result: ok")
	return 0
}

func runServe(parent context.Context, args []string, stdout, stderr io.Writer) int {
	path, ok := configPath(args)
	if !ok {
		fmt.Fprintln(stderr, "missing -c config.yaml")
		return 2
	}
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintln(stderr, "load config:", err)
		return 1
	}
	check := cfg.Check()
	if !check.OK() {
		for _, e := range check.Errors {
			fmt.Fprintln(stderr, "ERROR:", e)
		}
		return 1
	}
	logger, closer, err := logging.New(cfg.Logging)
	if err != nil {
		fmt.Fprintln(stderr, "logging:", err)
		return 1
	}
	defer closer.Close()
	slog.SetDefault(logger)
	logger.Info("aioproxy starting")
	store := storage.New(cfg.Storage.DataDir, cfg.Storage.SnapshotRetention)
	pool := core.NewPool()
	loaded, loadErr := store.LoadPool()
	if loadErr != nil {
		logger.Warn("load persisted pool degraded", "error", loadErr)
	}
	if len(loaded) > 0 {
		pool.Replace(loaded)
		logger.Info("loaded persisted candidates", "count", len(loaded))
	}
	sessions := core.NewSessionManager(cfg.Session.DefaultTTL.Duration, cfg.Session.MaxTTL.Duration)
	pm := newPluginManager(cfg, pool, store, logger)
	ctx, cancel := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	adminSrv := admin.New(cfg.Admin, pool, pm.Statuses, sessions, store, logger)
	if err := adminSrv.Start(); err != nil {
		logger.Error("admin start failed", "error", err)
		return 1
	}
	proxySrv := proxy.NewServer(*cfg, pool, sessions, logger)
	if err := proxySrv.Start(ctx); err != nil {
		logger.Error("proxy start failed", "error", err)
		return 1
	}
	pm.Start(ctx)
	fmt.Fprintf(stdout, "AIOPROXY serving proxy=%s admin=%s\n", cfg.Server.Listen, cfg.Admin.Listen)
	<-ctx.Done()
	logger.Info("shutdown signal received")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Lifecycle.GracePeriod.Duration)
	defer shutdownCancel()
	_ = proxySrv.Close()
	pm.Stop()
	proxySrv.Wait(cfg.Lifecycle.GracePeriod.Duration)
	if err := store.SavePool(pool.List()); err != nil {
		logger.Error("save pool failed", "error", err)
	}
	_ = adminSrv.Close(cfg.Lifecycle.GracePeriod.Duration)
	select {
	case <-shutdownCtx.Done():
	default:
	}
	logger.Info("aioproxy stopped")
	return 0
}

type pluginManager struct {
	cfg    *config.Config
	pool   *core.Pool
	store  *storage.Store
	logger *slog.Logger
	val    *validation.Validator
	items  []plugins.Plugin
	mu     sync.RWMutex
	status map[string]core.PluginStatus
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newPluginManager(cfg *config.Config, pool *core.Pool, store *storage.Store, logger *slog.Logger) *pluginManager {
	pm := &pluginManager{cfg: cfg, pool: pool, store: store, logger: logger, val: validation.New(cfg.Validation), status: map[string]core.PluginStatus{}}
	if cfg.Plugins.FPL != nil {
		pm.items = append(pm.items, fpl.New(*cfg.Plugins.FPL))
	}
	if cfg.Plugins.FOFA != nil && cfg.Plugins.FOFA.Key != "" {
		pm.items = append(pm.items, fofa.New(*cfg.Plugins.FOFA))
	}
	if cfg.Plugins.SingBox != nil && len(cfg.Plugins.SingBox.Sources) > 0 {
		pm.items = append(pm.items, singbox.New(*cfg.Plugins.SingBox))
	}
	for _, p := range pm.items {
		pm.status[p.Name()] = core.PluginStatus{Name: p.Name(), Active: true, Degraded: true, LastError: "not refreshed yet"}
	}
	return pm
}

func (pm *pluginManager) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	pm.cancel = cancel
	for _, p := range pm.items {
		plugin := p
		pm.wg.Add(1)
		go func() { defer pm.wg.Done(); pm.loop(ctx, plugin) }()
	}
}
func (pm *pluginManager) Stop() {
	if pm.cancel != nil {
		pm.cancel()
	}
	pm.wg.Wait()
	for _, p := range pm.items {
		if c, ok := p.(interface{ Close() }); ok {
			c.Close()
		}
	}
}
func (pm *pluginManager) Statuses() []core.PluginStatus {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make([]core.PluginStatus, 0, len(pm.status))
	for _, s := range pm.status {
		out = append(out, s)
	}
	return out
}

func (pm *pluginManager) loop(ctx context.Context, p plugins.Plugin) {
	pm.refresh(ctx, p)
	for {
		interval := p.RefreshInterval()
		if interval <= 0 {
			interval = time.Hour
		}
		jitter := time.Duration((rand.Float64()*2 - 1) * pm.cfg.Refresh.JitterRatio * float64(interval))
		t := time.NewTimer(interval + jitter)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			pm.refresh(ctx, p)
		}
	}
}

func (pm *pluginManager) refresh(ctx context.Context, p plugins.Plugin) {
	pm.logger.Info("plugin refresh started", "plugin", p.Name())
	res := p.Refresh(ctx)
	degraded := false
	lastErr := ""
	if len(res.Candidates) == 0 {
		degraded = true
	}
	for _, r := range res.Reports {
		if r.Error != "" {
			degraded = true
			lastErr = r.Error
		}
	}
	valid := pm.val.Validate(ctx, res.Candidates, res.Dialers)
	if resetIdleDialerCaches(res.Dialers) > 0 {
		debug.FreeOSMemory()
	}
	if len(res.Candidates) > 0 && len(valid) == 0 {
		degraded = true
		lastErr = "zero candidates passed validation"
	}
	pm.pool.AddValidated(valid, res.Dialers)
	updatedReports := make([]core.ImportReport, len(res.Reports))
	copy(updatedReports, res.Reports)
	for i := range updatedReports {
		reportCandidates := filterCandidatesForReport(valid, p.Name(), updatedReports[i].Source)
		updatedReports[i].Validated = len(reportCandidates)
		_ = pm.store.SaveSnapshot(p.Name()+"-"+updatedReports[i].Source, updatedReports[i], reportCandidates)
	}
	_ = pm.store.SavePool(pm.pool.List())
	pm.mu.Lock()
	pm.status[p.Name()] = core.PluginStatus{Name: p.Name(), Active: true, Degraded: degraded, LastRefresh: time.Now(), LastError: lastErr, Reports: updatedReports}
	pm.mu.Unlock()
	pm.logger.Info("plugin refresh finished", "plugin", p.Name(), "imported", len(res.Candidates), "validated", len(valid), "degraded", degraded)
}

func resetIdleDialerCaches(dialers map[string]core.CandidateDialer) int {
	reset := 0
	for _, d := range dialers {
		if resetter, ok := d.(interface{ ResetIdleCache() }); ok {
			resetter.ResetIdleCache()
			reset++
		}
	}
	return reset
}

func filterCandidatesForReport(candidates []core.Candidate, pluginName, reportSource string) []core.Candidate {
	out := make([]core.Candidate, 0, len(candidates))
	for _, c := range candidates {
		if c.Source != pluginName {
			continue
		}
		if reportSource == "" || c.Metadata["query"] == reportSource || c.Metadata["source"] == reportSource || c.Source == reportSource || len(candidates) == 1 {
			out = append(out, c)
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, c := range candidates {
		if c.Source == pluginName {
			out = append(out, c)
		}
	}
	return out
}
