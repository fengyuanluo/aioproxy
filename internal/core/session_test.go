package core

import (
	"fmt"
	"sync"
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

func TestSessionConcurrentFirstBindPinned(t *testing.T) {
	pool := NewPool()
	pool.AddValidated([]Candidate{
		{Protocol: ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "a"},
		{Protocol: ProtocolHTTP, Host: "2.2.2.2", Port: 80, Source: "b"},
	}, nil)
	sm := NewSessionManager(time.Minute, time.Hour)
	info := SessionInfo{Credential: "aio", SessionID: "job-001", TTL: time.Minute}

	start := make(chan struct{})
	var wg sync.WaitGroup
	seen := sync.Map{}
	for i := 0; i < 300; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			c, ok := sm.Pick(info, pool, "random")
			if !ok {
				t.Error("pick failed")
				return
			}
			seen.Store(c.Fingerprint, true)
		}()
	}
	close(start)
	wg.Wait()

	count := 0
	seen.Range(func(_, _ any) bool {
		count++
		return true
	})
	if count != 1 {
		t.Fatalf("same session should bind to one candidate, got %d", count)
	}
}

func TestRoundRobinStableOrder(t *testing.T) {
	pool := NewPool()
	pool.AddValidated([]Candidate{
		{Protocol: ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "a"},
		{Protocol: ProtocolHTTP, Host: "2.2.2.2", Port: 80, Source: "b"},
		{Protocol: ProtocolHTTP, Host: "3.3.3.3", Port: 80, Source: "c"},
	}, nil)
	want := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "1.1.1.1", "2.2.2.2", "3.3.3.3"}
	for i, w := range want {
		got, ok := pool.Pick("round_robin")
		if !ok {
			t.Fatalf("pick %d failed", i)
		}
		if got.Host != w {
			t.Fatalf("pick %d host=%s want=%s", i, got.Host, w)
		}
	}
}

func TestRoundRobinSkipsUnavailable(t *testing.T) {
	pool := NewPool()
	pool.AddValidated([]Candidate{
		{Protocol: ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "a"},
		{Protocol: ProtocolHTTP, Host: "2.2.2.2", Port: 80, Source: "b"},
	}, nil)
	first := pool.List()[0]
	pool.MarkFailure(first.Fingerprint, "boom", 1)
	for i := 0; i < 3; i++ {
		got, ok := pool.Pick("round_robin")
		if !ok {
			t.Fatalf("pick %d failed", i)
		}
		if got.Fingerprint == first.Fingerprint {
			t.Fatalf("unavailable candidate selected on pick %d", i)
		}
	}
}

func TestSessionPickSweepsExpired(t *testing.T) {
	pool := NewPool()
	pool.AddValidated([]Candidate{{Protocol: ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "a"}}, nil)
	sm := NewSessionManager(time.Millisecond, time.Hour)
	now := time.Now()
	for i := 0; i < 100; i++ {
		sm.bindings[fmt.Sprintf("old-%d", i)] = sessionBinding{Fingerprint: "missing", ExpiresAt: now.Add(-time.Hour), TTL: time.Millisecond}
	}
	sm.lastSweep = now.Add(-2 * time.Minute)
	if _, ok := sm.Pick(SessionInfo{SessionID: "new", TTL: time.Minute}, pool, "random"); !ok {
		t.Fatal("pick failed")
	}
	if got := len(sm.bindings); got != 1 {
		t.Fatalf("expired sessions should be swept during pick, got %d bindings", got)
	}
}
