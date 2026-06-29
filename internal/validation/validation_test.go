package validation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aioproxy/aioproxy/internal/config"
	"github.com/aioproxy/aioproxy/internal/core"
	"github.com/aioproxy/aioproxy/internal/proxy"
)

func TestValidateStopsDispatchWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v := New(config.ValidationConfig{URL: "http://127.0.0.1:1", Timeout: config.Duration{Duration: time.Second}, Concurrency: 1})
	candidates := make([]core.Candidate, 100)
	for i := range candidates {
		candidates[i] = core.Candidate{Protocol: core.ProtocolHTTP, Host: "127.0.0.1", Port: 1, Source: "test"}
	}
	start := time.Now()
	valid := v.Validate(ctx, candidates, nil)
	if len(valid) != 0 {
		t.Fatalf("valid=%d", len(valid))
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("canceled validation took too long: %v", elapsed)
	}
}

func TestValidateCandidateIPAPICountry(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","country":"United States","countryCode":"US","query":"1.2.3.4"}`))
	}))
	defer ts.Close()
	v := New(config.ValidationConfig{
		Strategy:    config.ValidationStrategyIPAPICountry,
		URL:         ts.URL,
		Timeout:     config.Duration{Duration: time.Second},
		Concurrency: 1,
	})
	cand := core.Candidate{Protocol: core.ProtocolSingBox, Source: "fofa"}
	got, err := v.ValidateCandidate(context.Background(), cand, proxy.DirectDialer{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Metadata["country_code"] != "US" || got.Metadata["country"] != "United States" || got.Metadata["query_ip"] != "1.2.3.4" {
		t.Fatalf("unexpected metadata: %+v", got.Metadata)
	}
}

func TestValidateRecordsLatencyOnSuccessfulCandidates(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()
	v := New(config.ValidationConfig{
		Strategy:      config.ValidationStrategyHTTPStatus,
		URL:           ts.URL,
		SuccessStatus: []int{http.StatusNoContent},
		Timeout:       config.Duration{Duration: time.Second},
		Concurrency:   1,
	})
	candidate := core.Candidate{Protocol: core.ProtocolSingBox, Source: "test"}
	candidate.Normalize()
	valid := v.Validate(context.Background(), []core.Candidate{candidate}, map[string]core.CandidateDialer{
		candidate.Fingerprint: proxy.DirectDialer{},
	})
	if len(valid) != 1 {
		t.Fatalf("valid=%d want=1", len(valid))
	}
	if valid[0].LastValidationLatency <= 0 {
		t.Fatalf("expected positive validation latency, got %v", valid[0].LastValidationLatency)
	}
	if valid[0].LastValidation.IsZero() {
		t.Fatal("expected last validation timestamp to be set")
	}
}
