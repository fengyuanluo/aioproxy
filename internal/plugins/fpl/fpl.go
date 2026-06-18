package fpl

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aioproxy/aioproxy/internal/config"
	"github.com/aioproxy/aioproxy/internal/core"
	"github.com/aioproxy/aioproxy/internal/proxy"
)

type Plugin struct {
	cfg    config.FPLConfig
	client *http.Client
}

func New(cfg config.FPLConfig) *Plugin {
	return &Plugin{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}}
}
func (p *Plugin) Name() string                   { return "fpl" }
func (p *Plugin) Active() bool                   { return true }
func (p *Plugin) RefreshInterval() time.Duration { return p.cfg.RefreshInterval.Duration }

func (p *Plugin) Refresh(ctx context.Context) core.PluginResult {
	started := time.Now()
	report := core.ImportReport{Plugin: p.Name(), Source: p.cfg.URL, StartedAt: started, SkipReasons: map[string]int{}}
	url := p.cfg.URL
	if url == "" {
		url = config.DefaultFPLURL
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := p.client.Do(req)
	if err != nil {
		report.Error = err.Error()
		report.FinishedAt = time.Now()
		return core.PluginResult{Reports: []core.ImportReport{report}}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		report.Error = resp.Status
		report.FinishedAt = time.Now()
		return core.PluginResult{Reports: []core.ImportReport{report}}
	}
	candidates := parse(resp.Body, &report)
	report.Imported = len(candidates)
	report.FinishedAt = time.Now()
	return core.PluginResult{Candidates: candidates, Reports: []core.ImportReport{report}}
}

func parse(r io.Reader, report *core.ImportReport) []core.Candidate {
	var out []core.Candidate
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		report.Total++
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://"):
			if c, ok := proxy.ParseHTTPProxyURL(line, "fpl"); ok {
				out = append(out, c)
			} else {
				report.AddSkip("invalid_http")
			}
		case strings.HasPrefix(lower, "socks5://") || strings.HasPrefix(lower, "socks://"):
			if strings.HasPrefix(lower, "socks://") {
				line = "socks5://" + line[len("socks://"):]
			}
			if c, ok := proxy.ParseSOCKSProxyURL(line, "fpl"); ok {
				out = append(out, c)
			} else {
				report.AddSkip("invalid_socks5")
			}
		case strings.HasPrefix(lower, "socks4://"):
			report.AddSkip("socks4_unsupported")
		default:
			report.AddSkip("unknown_scheme")
		}
	}
	return out
}
