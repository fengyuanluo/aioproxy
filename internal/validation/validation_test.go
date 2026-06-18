package validation

import (
	"context"
	"testing"
	"time"

	"github.com/aioproxy/aioproxy/internal/config"
	"github.com/aioproxy/aioproxy/internal/core"
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
