package config

import "testing"

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
