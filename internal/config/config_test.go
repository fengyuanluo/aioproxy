package config

import (
	"strings"
	"testing"
	"time"
)

func TestCheckAdminTokenRequiredForNonLoopback(t *testing.T) {
	c := Config{}
	c.ApplyDefaults()
	c.Auth.Enabled = true
	c.Admin.Listen = "0.0.0.0:1081"
	r := c.Check()
	if r.OK() {
		t.Fatal("expected error")
	}
}
func TestCheckWarningsNoActivePlugin(t *testing.T) {
	c := Config{}
	c.ApplyDefaults()
	c.Auth.Enabled = true
	r := c.Check()
	if !r.OK() {
		t.Fatalf("unexpected error: %v", r.Errors)
	}
	if len(r.Warnings) == 0 {
		t.Fatal("expected warning")
	}
}

func TestCheckRejectsOutOfRangePort(t *testing.T) {
	for _, tc := range []struct {
		name string
		mod  func(*Config)
		key  string
	}{
		{
			name: "server",
			mod: func(c *Config) {
				c.Server.Listen = "127.0.0.1:99999"
				c.Admin.Listen = "127.0.0.1:1081"
			},
			key: "server.listen",
		},
		{
			name: "admin",
			mod: func(c *Config) {
				c.Server.Listen = "127.0.0.1:1080"
				c.Admin.Listen = "127.0.0.1:99999"
			},
			key: "admin.listen",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Config{}
			c.ApplyDefaults()
			c.Auth.Enabled = true
			tc.mod(&c)
			r := c.Check()
			if r.OK() {
				t.Fatal("expected invalid listen port to fail check")
			}
			found := false
			for _, err := range r.Errors {
				if strings.Contains(err, tc.key) && strings.Contains(err, "invalid port") {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("unexpected errors: %v", r.Errors)
			}
		})
	}
}

func TestCheckAllowsZeroPort(t *testing.T) {
	c := Config{}
	c.ApplyDefaults()
	c.Auth.Enabled = true
	c.Server.Listen = "127.0.0.1:0"
	c.Admin.Listen = "127.0.0.1:1081"
	r := c.Check()
	if !r.OK() {
		t.Fatalf("expected zero port to be allowed, got errors: %v", r.Errors)
	}
}

func TestCheckRejectsUnknownValidationStrategy(t *testing.T) {
	c := Config{}
	c.ApplyDefaults()
	c.Validation.Strategy = "bad"
	r := c.Check()
	if r.OK() {
		t.Fatal("expected invalid validation strategy to fail")
	}
	found := false
	for _, err := range r.Errors {
		if strings.Contains(err, "validation.strategy") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("unexpected errors: %v", r.Errors)
	}
}

func TestCheckRejectsNegativeRetryAttempts(t *testing.T) {
	c := Config{}
	c.ApplyDefaults()
	c.RuntimeFailure.RetryAttempts = -1
	r := c.Check()
	if r.OK() {
		t.Fatal("expected negative retry_attempts to fail")
	}
	found := false
	for _, err := range r.Errors {
		if strings.Contains(err, "runtime_failure.retry_attempts") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("unexpected errors: %v", r.Errors)
	}
}

func TestSchedulerFastPoolPercentDefaultAndValidation(t *testing.T) {
	c := Config{}
	c.ApplyDefaults()
	if c.Scheduler.FastPoolPercent != 5 {
		t.Fatalf("scheduler.fast_pool_percent default=%d want=5", c.Scheduler.FastPoolPercent)
	}
	for _, tc := range []struct {
		value int
		ok    bool
	}{
		{1, true},
		{5, true},
		{100, true},
		{-1, false},
		{101, false},
	} {
		c := Config{}
		c.ApplyDefaults()
		c.Scheduler.FastPoolPercent = tc.value
		r := c.Check()
		if tc.ok && !r.OK() {
			t.Fatalf("value=%d expected ok, got errors=%v", tc.value, r.Errors)
		}
		if !tc.ok && r.OK() {
			t.Fatalf("value=%d expected failure", tc.value)
		}
	}
}

func TestSingBoxValidationConcurrencyDefaultAndValidation(t *testing.T) {
	c := Config{Plugins: PluginsConfig{SingBox: &SingBoxConfig{Sources: []SingBoxSourceConfig{{Name: "s", Type: "inline", URL: "outbounds: []"}}}}}
	c.ApplyDefaults()
	if c.Plugins.SingBox.ValidationConcurrency != 10 {
		t.Fatalf("singbox validation_concurrency default=%d want=10", c.Plugins.SingBox.ValidationConcurrency)
	}
	c.Plugins.SingBox.ValidationConcurrency = -1
	r := c.Check()
	if r.OK() {
		t.Fatal("expected negative singbox validation_concurrency to fail")
	}
	found := false
	for _, err := range r.Errors {
		if strings.Contains(err, "plugins.singbox.validation_concurrency") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("unexpected errors: %v", r.Errors)
	}
}

func TestCheckRejectsUnsafeTimingAndSizeBounds(t *testing.T) {
	tests := []struct {
		name string
		mod  func(*Config)
		key  string
	}{
		{
			name: "negative jitter",
			mod:  func(c *Config) { c.Refresh.JitterRatio = -0.1 },
			key:  "refresh.jitter_ratio",
		},
		{
			name: "too large jitter",
			mod:  func(c *Config) { c.Refresh.JitterRatio = 1.1 },
			key:  "refresh.jitter_ratio",
		},
		{
			name: "negative early failure window",
			mod:  func(c *Config) { c.RuntimeFailure.EarlyFailureWindow.Duration = -time.Second },
			key:  "runtime_failure.early_failure_window",
		},
		{
			name: "negative handshake timeout",
			mod:  func(c *Config) { c.Server.HandshakeTimeout.Duration = -time.Second },
			key:  "server.handshake_timeout",
		},
		{
			name: "validation concurrency too high",
			mod:  func(c *Config) { c.Validation.Concurrency = MaxValidationConcurrency + 1 },
			key:  "validation.concurrency",
		},
		{
			name: "fofa size too high",
			mod: func(c *Config) {
				c.Plugins.FOFA = &FOFAConfig{Key: "sample", Size: MaxFOFASize + 1}
			},
			key: "plugins.fofa.size",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := Config{}
			c.ApplyDefaults()
			tc.mod(&c)
			r := c.Check()
			if r.OK() {
				t.Fatal("expected config check to fail")
			}
			found := false
			for _, err := range r.Errors {
				if strings.Contains(err, tc.key) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected error containing %q, got %v", tc.key, r.Errors)
			}
		})
	}
}
