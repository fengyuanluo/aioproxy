package fpl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aioproxy/aioproxy/internal/config"
	"github.com/aioproxy/aioproxy/internal/core"
)

func TestParseFPL(t *testing.T) {
	rep := &core.ImportReport{}
	c := parse(strings.NewReader("http://1.2.3.4:80\nsocks5://5.6.7.8:1080\nsocks4://1.1.1.1:1\n"), rep)
	if len(c) != 2 {
		t.Fatalf("got %d", len(c))
	}
	if rep.SkipReasons["socks4_unsupported"] != 1 {
		t.Fatalf("skip=%v", rep.SkipReasons)
	}
}

func TestSourceLabelDoesNotExposeURLCredentials(t *testing.T) {
	label := sourceLabel("http://sample-user:sample-pass@proxy-source.example.invalid:18083/list.txt?sample-token=redacted")
	for _, forbidden := range []string{"sample-user", "sample-pass", "sample-token", "redacted", "list.txt"} {
		if strings.Contains(label, forbidden) {
			t.Fatalf("label %q contains forbidden substring %q", label, forbidden)
		}
	}
	if !strings.HasPrefix(label, "url-proxy-source.example.invalid:18083-") {
		t.Fatalf("unexpected label %q", label)
	}
}

func TestRefreshInvalidURLReturnsReportError(t *testing.T) {
	p := New(config.FPLConfig{URL: "http://%zz"})
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

func TestRefreshAnnotatesCandidatesWithSourceLabel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("http://1.2.3.4:80\n"))
	}))
	defer ts.Close()
	p := New(config.FPLConfig{URL: ts.URL})
	res := p.Refresh(context.Background())
	if len(res.Candidates) != 1 {
		t.Fatalf("candidates=%d reports=%+v", len(res.Candidates), res.Reports)
	}
	if len(res.Reports) != 1 {
		t.Fatalf("reports=%d", len(res.Reports))
	}
	if got, want := res.Candidates[0].Metadata["source"], res.Reports[0].Source; got != want || got == "" {
		t.Fatalf("candidate source metadata=%q want report source %q", got, want)
	}
}

func TestParseReportsScannerError(t *testing.T) {
	rep := &core.ImportReport{}
	_ = parse(strings.NewReader("http://"+strings.Repeat("a", 2<<20)+":80\n"), rep)
	if rep.Error == "" {
		t.Fatalf("expected scanner error for overlong line")
	}
	if !strings.Contains(rep.Error, "scan proxy list") {
		t.Fatalf("unexpected error: %s", rep.Error)
	}
}
