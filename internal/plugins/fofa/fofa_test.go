package fofa

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aioproxy/aioproxy/internal/config"
)

func TestSearchRejectsOversizedResponse(t *testing.T) {
	old := maxFOFAResponseBytes
	maxFOFAResponseBytes = 64
	defer func() { maxFOFAResponseBytes = old }()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", int(maxFOFAResponseBytes)+1)))
	}))
	defer ts.Close()
	p := New(config.FOFAConfig{
		BaseURL:         ts.URL,
		Key:             "sample",
		Size:            1,
		RefreshInterval: config.Duration{Duration: time.Hour},
		Queries:         []config.FOFAQueryConfig{{Name: "q", Protocol: "http", Query: "q", Fields: "ip,port"}},
	})
	res := p.Refresh(context.Background())
	if len(res.Reports) != 1 || res.Reports[0].Error == "" {
		t.Fatalf("expected oversized response report error, got %+v", res.Reports)
	}
	if !strings.Contains(res.Reports[0].Error, "too large") {
		t.Fatalf("unexpected error: %s", res.Reports[0].Error)
	}
}
