package config

import (
	"strings"
	"testing"
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
