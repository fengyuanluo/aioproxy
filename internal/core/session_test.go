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
		fast        bool
	}{{"aio", "", 0, false}, {"aio-fast", "", 0, true}, {"aio-job001", "job001", def, false}, {"aio-job001-fast", "job001", def, true}, {"aio-job-001", "job-001", def, false}, {"aio-job-001-30m", "job-001", 30 * time.Minute, false}, {"aio-job-001-30m-fast", "job-001", 30 * time.Minute, true}, {"aio-job-001-99h", "job-001", max, false}}
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
		if got.Fast != tc.fast {
			t.Fatalf("%s fast=%v want=%v", tc.in, got.Fast, tc.fast)
		}
	}
	if _, ok := ParseSessionUsername("bob-job", "aio", def, max); ok {
		t.Fatal("unexpected auth ok")
	}
}

func TestParseStructuredUsername(t *testing.T) {
	def := 30 * time.Minute
	max := time.Hour
	got, ok := ParseSessionUsername("aio~plugin=fofa~region=us~session=job-001~ttl=99h", "aio", def, max)
	if !ok {
		t.Fatal("expected structured username to parse")
	}
	if got.Plugin != "fofa" || got.Region != "US" || got.SessionID != "job-001" || got.TTL != max {
		t.Fatalf("unexpected parse result: %+v", got)
	}
	got, ok = ParseSessionUsername("aio~ttl=30m~session=job-002", "aio", def, max)
	if !ok || got.SessionID != "job-002" || got.TTL != 30*time.Minute {
		t.Fatalf("expected ttl-before-session ordering support, got %+v ok=%v", got, ok)
	}
	got, ok = ParseSessionUsername("aio~fast=true~plugin=fpl~session=job-003~ttl=30m", "aio", def, max)
	if !ok || !got.Fast || got.Plugin != "fpl" || got.SessionID != "job-003" {
		t.Fatalf("expected structured fast username, got %+v ok=%v", got, ok)
	}
}

func TestParseStructuredUsernameRejectsInvalid(t *testing.T) {
	def := 30 * time.Minute
	max := time.Hour
	cases := []string{
		"aio~plugin=",
		"aio~unknown=x",
		"aio~ttl=30m",
		"aio~fast=false",
		"aio~fast=1",
		"aio~plugin=fofa~plugin=fpl",
		"aio~fast=true~fast=true",
		"bob~plugin=fofa",
	}
	for _, in := range cases {
		if _, ok := ParseSessionUsername(in, "aio", def, max); ok {
			t.Fatalf("expected %q to fail", in)
		}
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

func TestSessionScopedBindingsDoNotCrossPlugin(t *testing.T) {
	pool := NewPool()
	pool.AddValidated([]Candidate{
		{Protocol: ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "fofa"},
		{Protocol: ProtocolHTTP, Host: "2.2.2.2", Port: 80, Source: "fpl"},
	}, nil)
	sm := NewSessionManager(time.Minute, time.Hour)
	fofaInfo := SessionInfo{Credential: "aio", SessionID: "job-001", TTL: time.Minute, Plugin: "fofa"}
	fplInfo := SessionInfo{Credential: "aio", SessionID: "job-001", TTL: time.Minute, Plugin: "fpl"}
	first, ok := sm.Pick(fofaInfo, pool, "random")
	if !ok || first.Source != "fofa" {
		t.Fatalf("expected fofa candidate, got %+v ok=%v", first, ok)
	}
	second, ok := sm.Pick(fplInfo, pool, "random")
	if !ok || second.Source != "fpl" {
		t.Fatalf("expected fpl candidate, got %+v ok=%v", second, ok)
	}
}

func TestSessionScopedBindingsDoNotCrossFastPool(t *testing.T) {
	pool := NewPool()
	pool.AddValidated([]Candidate{
		{Protocol: ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "fpl"},
		{Protocol: ProtocolHTTP, Host: "2.2.2.2", Port: 80, Source: "fpl"},
	}, nil)
	sm := NewSessionManager(time.Minute, time.Hour)
	normalInfo := SessionInfo{Credential: "aio", SessionID: "job-001", TTL: time.Minute, Plugin: "fpl", Fast: false}
	fastInfo := SessionInfo{Credential: "aio", SessionID: "job-001", TTL: time.Minute, Plugin: "fpl", Fast: true}
	list := pool.List()
	sm.Rebind(normalInfo, list[0].Fingerprint)
	sm.Rebind(fastInfo, list[1].Fingerprint)
	gotNormal, ok := sm.GetBound(normalInfo, pool)
	if !ok || gotNormal.Fingerprint != list[0].Fingerprint {
		t.Fatalf("expected normal binding on first candidate, got %+v ok=%v", gotNormal, ok)
	}
	gotFast, ok := sm.GetBound(fastInfo, pool)
	if !ok || gotFast.Fingerprint != list[1].Fingerprint {
		t.Fatalf("expected fast binding on second candidate, got %+v ok=%v", gotFast, ok)
	}
}

func TestSessionRebindAndUnbind(t *testing.T) {
	pool := NewPool()
	pool.AddValidated([]Candidate{
		{Protocol: ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "a"},
		{Protocol: ProtocolHTTP, Host: "2.2.2.2", Port: 80, Source: "a"},
	}, nil)
	list := pool.List()
	sm := NewSessionManager(time.Minute, time.Hour)
	info := SessionInfo{Credential: "aio", SessionID: "job-001", TTL: time.Minute}

	sm.Rebind(info, list[0].Fingerprint)
	got, ok := sm.GetBound(info, pool)
	if !ok || got.Fingerprint != list[0].Fingerprint {
		t.Fatalf("expected initial binding, got %+v ok=%v", got, ok)
	}

	sm.Rebind(info, list[1].Fingerprint)
	got, ok = sm.GetBound(info, pool)
	if !ok || got.Fingerprint != list[1].Fingerprint {
		t.Fatalf("expected rebound candidate, got %+v ok=%v", got, ok)
	}

	sm.Unbind(info)
	if _, ok := sm.GetBound(info, pool); ok {
		t.Fatal("expected binding to be removed")
	}
}
