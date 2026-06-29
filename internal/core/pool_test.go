package core

import (
	"context"
	"fmt"
	"net"
	"sort"
	"testing"
	"time"
)

func TestRandomPickLargePoolAvoidsPerPickAllocation(t *testing.T) {
	pool := NewPool()
	candidates := make([]Candidate, 10_000)
	for i := range candidates {
		candidates[i] = Candidate{Protocol: ProtocolHTTP, Host: fmt.Sprintf("10.%d.%d.%d", i/65536, (i/256)%256, i%256), Port: 8080, Source: "bench"}
	}
	pool.AddValidated(candidates, nil)

	allocs := testing.AllocsPerRun(1_000, func() {
		if _, ok := pool.Pick("random"); !ok {
			t.Fatal("random pick failed")
		}
	})
	if allocs != 0 {
		t.Fatalf("random pick allocations/op = %.2f, want 0; do not rebuild a full available slice on the hot path", allocs)
	}
}

func TestRandomPickHonorsMatchAndExclude(t *testing.T) {
	pool := NewPool()
	pool.AddValidated([]Candidate{
		{Protocol: ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "fpl", Metadata: map[string]string{"country_code": "US"}},
		{Protocol: ProtocolHTTP, Host: "2.2.2.2", Port: 80, Source: "fpl", Metadata: map[string]string{"country_code": "JP"}},
		{Protocol: ProtocolHTTP, Host: "3.3.3.3", Port: 80, Source: "fofa", Metadata: map[string]string{"country_code": "US"}},
	}, nil)
	list := pool.List()
	exclude := map[string]struct{}{
		list[0].Fingerprint: {},
		list[2].Fingerprint: {},
	}
	for i := 0; i < 50; i++ {
		got, ok := pool.PickMatchingExcluding("random", func(c Candidate) bool {
			return c.MatchesRoute("fpl", "")
		}, exclude)
		if !ok {
			t.Fatalf("pick %d failed", i)
		}
		if got.Fingerprint != list[1].Fingerprint {
			t.Fatalf("pick %d got fingerprint %s host=%s, want only non-excluded fpl candidate %s", i, got.Fingerprint, got.Host, list[1].Fingerprint)
		}
	}
}

func TestPickMatchingPercentExcludingKeepsFastestRouteScopedCandidates(t *testing.T) {
	pool := NewPool()
	pool.AddValidated([]Candidate{
		{Protocol: ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "fpl", LastValidationLatency: 10 * time.Millisecond},
		{Protocol: ProtocolHTTP, Host: "2.2.2.2", Port: 80, Source: "fpl", LastValidationLatency: 20 * time.Millisecond},
		{Protocol: ProtocolHTTP, Host: "3.3.3.3", Port: 80, Source: "fpl", LastValidationLatency: 30 * time.Millisecond},
		{Protocol: ProtocolHTTP, Host: "4.4.4.4", Port: 80, Source: "fofa", LastValidationLatency: time.Millisecond},
	}, nil)
	for i := 0; i < 50; i++ {
		got, ok := pool.PickMatchingPercentExcluding("random", func(c Candidate) bool {
			return c.MatchesRoute("fpl", "")
		}, nil, 34)
		if !ok {
			t.Fatal("expected fast pick to succeed")
		}
		if got.Host != "1.1.1.1" {
			t.Fatalf("got host=%s want fastest fpl candidate", got.Host)
		}
	}
}

func TestPickMatchingPercentExcludingRoundRobinUsesFastSubset(t *testing.T) {
	pool := NewPool()
	pool.AddValidated([]Candidate{
		{Protocol: ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "fpl", LastValidationLatency: 10 * time.Millisecond},
		{Protocol: ProtocolHTTP, Host: "2.2.2.2", Port: 80, Source: "fpl", LastValidationLatency: 20 * time.Millisecond},
		{Protocol: ProtocolHTTP, Host: "3.3.3.3", Port: 80, Source: "fpl", LastValidationLatency: 30 * time.Millisecond},
		{Protocol: ProtocolHTTP, Host: "4.4.4.4", Port: 80, Source: "fpl", LastValidationLatency: 40 * time.Millisecond},
	}, nil)
	want := []string{"1.1.1.1", "2.2.2.2", "1.1.1.1", "2.2.2.2"}
	for i, host := range want {
		got, ok := pool.PickMatchingPercentExcluding("round_robin", func(c Candidate) bool { return c.MatchesRoute("fpl", "") }, nil, 50)
		if !ok {
			t.Fatalf("pick %d failed", i)
		}
		if got.Host != host {
			t.Fatalf("pick %d host=%s want=%s", i, got.Host, host)
		}
	}
}

func TestPickMatchingPercentExcludingKeepsAtLeastOneCandidate(t *testing.T) {
	pool := NewPool()
	pool.AddValidated([]Candidate{
		{Protocol: ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "fpl", LastValidationLatency: 20 * time.Millisecond},
		{Protocol: ProtocolHTTP, Host: "2.2.2.2", Port: 80, Source: "fpl", LastValidationLatency: 10 * time.Millisecond},
	}, nil)
	got, ok := pool.PickMatchingPercentExcluding("random", func(c Candidate) bool { return c.MatchesRoute("fpl", "") }, nil, 5)
	if !ok {
		t.Fatal("expected at least one fast candidate")
	}
	if got.Host != "2.2.2.2" {
		t.Fatalf("got host=%s want fastest candidate", got.Host)
	}
}

func TestPickMatchingPercentExcludingPushesUnknownLatencyToEnd(t *testing.T) {
	pool := NewPool()
	pool.AddValidated([]Candidate{
		{Protocol: ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "fpl"},
		{Protocol: ProtocolHTTP, Host: "2.2.2.2", Port: 80, Source: "fpl", LastValidationLatency: 5 * time.Millisecond},
		{Protocol: ProtocolHTTP, Host: "3.3.3.3", Port: 80, Source: "fpl", LastValidationLatency: 10 * time.Millisecond},
	}, nil)
	seen := make(map[string]struct{})
	for i := 0; i < 50; i++ {
		got, ok := pool.PickMatchingPercentExcluding("random", func(c Candidate) bool { return c.MatchesRoute("fpl", "") }, nil, 67)
		if !ok {
			t.Fatal("expected fast pick")
		}
		seen[got.Host] = struct{}{}
		if got.Host == "1.1.1.1" {
			t.Fatal("candidate with unknown latency should not enter truncated fast subset")
		}
	}
	if len(seen) != 2 {
		keys := make([]string, 0, len(seen))
		for k := range seen {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		t.Fatalf("expected only the two timed candidates, saw %v", keys)
	}
}

func BenchmarkPoolRandomPickLarge(b *testing.B) {
	pool := NewPool()
	candidates := make([]Candidate, 10_000)
	for i := range candidates {
		candidates[i] = Candidate{Protocol: ProtocolHTTP, Host: fmt.Sprintf("10.%d.%d.%d", i/65536, (i/256)%256, i%256), Port: 8080, Source: "bench"}
	}
	pool.AddValidated(candidates, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := pool.Pick("random"); !ok {
			b.Fatal("random pick failed")
		}
	}
}

func TestReplaceValidatedMatchingRemovesStaleCandidatesAndDialers(t *testing.T) {
	pool := NewPool()
	pool.AddValidated([]Candidate{
		{Protocol: ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "fpl", Metadata: map[string]string{"source": "default"}},
		{Protocol: ProtocolHTTP, Host: "2.2.2.2", Port: 80, Source: "fpl", Metadata: map[string]string{"source": "default"}},
		{Protocol: ProtocolHTTP, Host: "3.3.3.3", Port: 80, Source: "fofa", Metadata: map[string]string{"query": "http-proxy"}},
	}, nil)
	list := pool.List()
	staleDialer := &resettableDialer{}
	keptDialer := &resettableDialer{}
	pool.RegisterDialer(list[0].Fingerprint, staleDialer)
	pool.RegisterDialer(list[1].Fingerprint, keptDialer)

	kept := list[1]
	kept.LastError = "old error"
	kept.FailureCount = 2
	added := pool.ReplaceValidatedMatching([]Candidate{kept}, map[string]CandidateDialer{kept.Fingerprint: keptDialer}, func(c Candidate) bool {
		return c.Source == "fpl" && c.Metadata["source"] == "default"
	})
	if added != 1 {
		t.Fatalf("added=%d want=1", added)
	}
	if _, ok := pool.Get(list[0].Fingerprint); ok {
		t.Fatalf("stale candidate still present")
	}
	if d := pool.Dialer(list[0].Fingerprint); d != nil {
		t.Fatalf("stale dialer still present: %#v", d)
	}
	if !staleDialer.reset {
		t.Fatalf("stale resettable dialer was not reset before deletion")
	}
	got, ok := pool.GetAvailable(kept.Fingerprint)
	if !ok {
		t.Fatalf("kept candidate not available")
	}
	if got.FailureCount != 0 || got.LastError != "" {
		t.Fatalf("kept candidate runtime failure state not reset: %+v", got)
	}
	if _, ok := pool.Get(list[2].Fingerprint); !ok {
		t.Fatalf("unmatched fofa candidate was removed")
	}
}

type resettableDialer struct{ reset bool }

func (d *resettableDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *resettableDialer) ResetIdleCache() { d.reset = true }
