package core

import (
	"testing"
	"time"
)

func TestParseSessionUsername(t *testing.T) {
	def := 30 * time.Minute
	max := time.Hour
	cases := []struct {
		in, session string
		ttl         time.Duration
	}{{"aio", "", 0}, {"aio-job001", "job001", def}, {"aio-job-001", "job-001", def}, {"aio-job-001-30m", "job-001", 30 * time.Minute}, {"aio-job-001-99h", "job-001", max}}
	for _, tc := range cases {
		got, ok := ParseSessionUsername(tc.in, "aio", def, max)
		if !ok {
			t.Fatalf("%s not ok", tc.in)
		}
		if got.SessionID != tc.session {
			t.Fatalf("%s session=%q", tc.in, got.SessionID)
		}
		if tc.ttl != 0 && got.TTL != tc.ttl {
			t.Fatalf("%s ttl=%v", tc.in, got.TTL)
		}
	}
	if _, ok := ParseSessionUsername("bob-job", "aio", def, max); ok {
		t.Fatal("unexpected auth ok")
	}
}
