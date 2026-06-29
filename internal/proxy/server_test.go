package proxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aioproxy/aioproxy/internal/config"
	"github.com/aioproxy/aioproxy/internal/core"
)

func TestMixedHTTPAndSOCKS5Proxy(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
	defer origin.Close()
	u, _ := url.Parse(origin.URL)
	host, portStr, _ := net.SplitHostPort(u.Host)
	port, _ := strconv.Atoi(portStr)
	pool := core.NewPool()
	pool.AddValidated([]core.Candidate{{Protocol: core.ProtocolSingBox, Host: host, Port: port, Source: "test"}}, map[string]core.CandidateDialer{})
	c := pool.List()[0]
	pool.RegisterDialer(c.Fingerprint, DirectDialer{})
	cfg := config.Config{}
	cfg.ApplyDefaults()
	cfg.Server.Listen = "127.0.0.1:0"
	cfg.Auth.Enabled = true
	cfg.RuntimeFailure.MaxFailures = 3
	cfg.RuntimeFailure.EarlyFailureWindow.Duration = time.Second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := NewServer(cfg, pool, core.NewSessionManager(time.Minute, time.Hour), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	proxyURL, _ := url.Parse("http://aio:change-me@" + srv.Addr())
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	resp, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "ok" {
		t.Fatalf("body=%q", body)
	}
	if err := socksGet(srv.Addr(), "aio", "change-me", u.Host); err != nil {
		t.Fatal(err)
	}
}

func TestProxy300Concurrent(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
	defer origin.Close()
	u, _ := url.Parse(origin.URL)
	host, portStr, _ := net.SplitHostPort(u.Host)
	port, _ := strconv.Atoi(portStr)
	pool := core.NewPool()
	pool.AddValidated([]core.Candidate{{Protocol: core.ProtocolSingBox, Host: host, Port: port, Source: "stress"}}, nil)
	c := pool.List()[0]
	pool.RegisterDialer(c.Fingerprint, DirectDialer{})
	cfg := config.Config{}
	cfg.ApplyDefaults()
	cfg.Server.Listen = "127.0.0.1:0"
	cfg.Auth.Enabled = true
	cfg.RuntimeFailure.MaxFailures = 3
	cfg.RuntimeFailure.EarlyFailureWindow.Duration = time.Second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := NewServer(cfg, pool, core.NewSessionManager(time.Minute, time.Hour), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	proxyURL, _ := url.Parse("http://aio:change-me@" + srv.Addr())
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL), MaxIdleConnsPerHost: 400}, Timeout: 10 * time.Second}
	var wg sync.WaitGroup
	errCh := make(chan error, 300)
	for i := 0; i < 300; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(origin.URL)
			if err != nil {
				errCh <- err
				return
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if string(b) != "ok" {
				errCh <- fmt.Errorf("bad body %q", b)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestHTTPProxyUsernameRouteByPluginAndRegion(t *testing.T) {
	fofaOrigin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "fofa") }))
	defer fofaOrigin.Close()
	usOrigin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "us") }))
	defer usOrigin.Close()
	fofaURL, _ := url.Parse(fofaOrigin.URL)
	usURL, _ := url.Parse(usOrigin.URL)
	fofaHost, fofaPortStr, _ := net.SplitHostPort(fofaURL.Host)
	usHost, usPortStr, _ := net.SplitHostPort(usURL.Host)
	fofaPort, _ := strconv.Atoi(fofaPortStr)
	usPort, _ := strconv.Atoi(usPortStr)

	pool := core.NewPool()
	pool.AddValidated([]core.Candidate{
		{Protocol: core.ProtocolSingBox, Host: fofaHost, Port: fofaPort, Source: "fofa", Metadata: map[string]string{"country_code": "JP", "tag": "fofa-jp"}},
		{Protocol: core.ProtocolSingBox, Host: usHost, Port: usPort, Source: "singbox", Metadata: map[string]string{"country_code": "US", "tag": "singbox-us"}},
	}, nil)
	list := pool.List()
	pool.RegisterDialer(list[0].Fingerprint, fixedTargetDialer{target: mustHostPort(t, fofaOrigin.URL)})
	pool.RegisterDialer(list[1].Fingerprint, fixedTargetDialer{target: mustHostPort(t, usOrigin.URL)})

	cfg := config.Config{}
	cfg.ApplyDefaults()
	cfg.Server.Listen = "127.0.0.1:0"
	cfg.Auth.Enabled = true
	cfg.RuntimeFailure.MaxFailures = 3
	cfg.RuntimeFailure.EarlyFailureWindow.Duration = time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := NewServer(cfg, pool, core.NewSessionManager(time.Minute, time.Hour), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	body := httpViaProxy(t, srv.Addr(), "aio~plugin=fofa", "change-me", "http://example.test/")
	if body != "fofa" {
		t.Fatalf("plugin route body=%q", body)
	}
	body = httpViaProxy(t, srv.Addr(), "aio~region=US", "change-me", "http://example.test/")
	if body != "us" {
		t.Fatalf("region route body=%q", body)
	}
}

func TestHTTPProxyRetriesWithinRouteAndSucceeds(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
	defer origin.Close()
	target := mustHostPort(t, origin.URL)

	pool := core.NewPool()
	pool.AddValidated([]core.Candidate{
		{Protocol: core.ProtocolSingBox, Host: "1.1.1.1", Port: 80, Source: "fpl"},
		{Protocol: core.ProtocolSingBox, Host: "2.2.2.2", Port: 80, Source: "fpl"},
	}, nil)
	list := pool.List()
	pool.RegisterDialer(list[0].Fingerprint, failDialer{err: errors.New("boom")})
	pool.RegisterDialer(list[1].Fingerprint, fixedTargetDialer{target: target})

	cfg := config.Config{}
	cfg.ApplyDefaults()
	cfg.Server.Listen = "127.0.0.1:0"
	cfg.Auth.Enabled = true
	cfg.Scheduler.Policy = "round_robin"
	cfg.RuntimeFailure.MaxFailures = 3
	cfg.RuntimeFailure.RetryAttempts = 1
	cfg.RuntimeFailure.EarlyFailureWindow.Duration = time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := NewServer(cfg, pool, core.NewSessionManager(time.Minute, time.Hour), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	body := httpViaProxy(t, srv.Addr(), "aio~plugin=fpl", "change-me", origin.URL)
	if body != "ok" {
		t.Fatalf("body=%q", body)
	}
	first, _ := pool.Get(list[0].Fingerprint)
	if first.FailureCount != 1 {
		t.Fatalf("expected first candidate failure count=1, got %d", first.FailureCount)
	}
}

func TestSessionRetryRebindsToHealthyCandidate(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
	defer origin.Close()
	target := mustHostPort(t, origin.URL)

	pool := core.NewPool()
	pool.AddValidated([]core.Candidate{
		{Protocol: core.ProtocolSingBox, Host: "1.1.1.1", Port: 80, Source: "fpl"},
		{Protocol: core.ProtocolSingBox, Host: "2.2.2.2", Port: 80, Source: "fpl"},
	}, nil)
	list := pool.List()
	pool.RegisterDialer(list[0].Fingerprint, failDialer{err: errors.New("boom")})
	pool.RegisterDialer(list[1].Fingerprint, fixedTargetDialer{target: target})

	sm := core.NewSessionManager(time.Minute, time.Hour)
	info := core.SessionInfo{Credential: "aio", SessionID: "job-001", TTL: time.Minute, Plugin: "fpl"}
	sm.Rebind(info, list[0].Fingerprint)

	cfg := config.Config{}
	cfg.ApplyDefaults()
	cfg.Server.Listen = "127.0.0.1:0"
	cfg.Auth.Enabled = true
	cfg.Scheduler.Policy = "round_robin"
	cfg.RuntimeFailure.MaxFailures = 3
	cfg.RuntimeFailure.RetryAttempts = 1
	cfg.RuntimeFailure.EarlyFailureWindow.Duration = time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := NewServer(cfg, pool, sm, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	body := httpViaProxy(t, srv.Addr(), "aio~plugin=fpl~session=job-001", "change-me", origin.URL)
	if body != "ok" {
		t.Fatalf("body=%q", body)
	}
	bound, ok := sm.GetBound(info, pool)
	if !ok || bound.Fingerprint != list[1].Fingerprint {
		t.Fatalf("expected rebound to healthy candidate, got %+v ok=%v", bound, ok)
	}
}

func TestHTTPProxyRouteFailureDoesNotFallbackGlobalPool(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
	defer origin.Close()
	target := mustHostPort(t, origin.URL)

	pool := core.NewPool()
	pool.AddValidated([]core.Candidate{
		{Protocol: core.ProtocolSingBox, Host: "1.1.1.1", Port: 80, Source: "fofa", Metadata: map[string]string{"country_code": "JP"}},
		{Protocol: core.ProtocolSingBox, Host: "2.2.2.2", Port: 80, Source: "fpl", Metadata: map[string]string{"country_code": "US"}},
	}, nil)
	list := pool.List()
	pool.RegisterDialer(list[0].Fingerprint, failDialer{err: errors.New("fofa down")})
	pool.RegisterDialer(list[1].Fingerprint, fixedTargetDialer{target: target})

	cfg := config.Config{}
	cfg.ApplyDefaults()
	cfg.Server.Listen = "127.0.0.1:0"
	cfg.Auth.Enabled = true
	cfg.Scheduler.Policy = "round_robin"
	cfg.RuntimeFailure.MaxFailures = 3
	cfg.RuntimeFailure.RetryAttempts = 2
	cfg.RuntimeFailure.EarlyFailureWindow.Duration = time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := NewServer(cfg, pool, core.NewSessionManager(time.Minute, time.Hour), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	proxyURL, err := url.Parse("http://" + url.UserPassword("aio~plugin=fofa", "change-me").String() + "@" + srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 5 * time.Second}
	resp, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%q", resp.StatusCode, body)
	}
}

func TestHTTPProxyKeepsClientConnectionForSequentialRequests(t *testing.T) {
	var hits atomic.Int64
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok-%d", hits.Add(1))
	}))
	defer origin.Close()
	target := mustHostPort(t, origin.URL)
	pool := core.NewPool()
	pool.AddValidated([]core.Candidate{{Protocol: core.ProtocolSingBox, Host: "1.1.1.1", Port: 80, Source: "test"}}, nil)
	c := pool.List()[0]
	pool.RegisterDialer(c.Fingerprint, fixedTargetDialer{target: target})
	cfg := config.Config{}
	cfg.ApplyDefaults()
	cfg.Server.Listen = "127.0.0.1:0"
	cfg.Auth.Enabled = true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := NewServer(cfg, pool, core.NewSessionManager(time.Minute, time.Hour), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	br := bufio.NewReader(conn)
	auth := "Proxy-Authorization: Basic YWlvOmNoYW5nZS1tZQ==\r\n"
	for i := 1; i <= 2; i++ {
		if _, err := fmt.Fprintf(conn, "GET http://example.test/request-%d HTTP/1.1\r\nHost: example.test\r\n%s\r\n", i, auth); err != nil {
			t.Fatal(err)
		}
		resp, err := http.ReadResponse(br, nil)
		if err != nil {
			t.Fatalf("read response %d: %v", i, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if string(body) != fmt.Sprintf("ok-%d", i) {
			t.Fatalf("response %d body=%q", i, body)
		}
	}
}

func TestTunnelIdlePastEarlyWindowDoesNotMarkFailure(t *testing.T) {
	pool := core.NewPool()
	pool.AddValidated([]core.Candidate{{Protocol: core.ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "test"}}, nil)
	cand := pool.List()[0]
	cfg := config.Config{}
	cfg.ApplyDefaults()
	cfg.RuntimeFailure.MaxFailures = 1
	cfg.RuntimeFailure.EarlyFailureWindow.Duration = 20 * time.Millisecond
	srv := NewServer(cfg, pool, core.NewSessionManager(time.Minute, time.Hour), slog.New(slog.NewTextHandler(io.Discard, nil)))
	clientSide, clientPeer := net.Pipe()
	upstreamSide, upstreamPeer := net.Pipe()
	defer clientPeer.Close()
	defer upstreamPeer.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.tunnel(clientSide, upstreamSide, cand)
	}()
	time.Sleep(3 * cfg.RuntimeFailure.EarlyFailureWindow.Duration)
	got, ok := pool.Get(cand.Fingerprint)
	if !ok {
		t.Fatal("candidate missing")
	}
	if got.FailureCount != 0 || got.Status != core.StatusAvailable {
		t.Fatalf("idle open tunnel was marked failed: %+v", got)
	}
	_ = clientPeer.Close()
	_ = upstreamPeer.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("tunnel did not exit")
	}
	got, _ = pool.Get(cand.Fingerprint)
	if got.FailureCount != 0 || got.Status != core.StatusAvailable {
		t.Fatalf("idle tunnel close after early window was marked failed: %+v", got)
	}
}

func TestTunnelEarlyZeroByteCloseMarksFailure(t *testing.T) {
	pool := core.NewPool()
	pool.AddValidated([]core.Candidate{{Protocol: core.ProtocolHTTP, Host: "1.1.1.1", Port: 80, Source: "test"}}, nil)
	cand := pool.List()[0]
	cfg := config.Config{}
	cfg.ApplyDefaults()
	cfg.RuntimeFailure.MaxFailures = 1
	cfg.RuntimeFailure.EarlyFailureWindow.Duration = time.Second
	srv := NewServer(cfg, pool, core.NewSessionManager(time.Minute, time.Hour), slog.New(slog.NewTextHandler(io.Discard, nil)))
	clientSide, clientPeer := net.Pipe()
	upstreamSide, upstreamPeer := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.tunnel(clientSide, upstreamSide, cand)
	}()
	_ = clientPeer.Close()
	_ = upstreamPeer.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("tunnel did not exit")
	}
	got, _ := pool.Get(cand.Fingerprint)
	if got.FailureCount != 1 || got.Status != core.StatusUnavailable {
		t.Fatalf("early zero-byte close was not marked failed: %+v", got)
	}
}

func TestSlowInitialHandshakeTimesOut(t *testing.T) {
	pool := core.NewPool()
	cfg := config.Config{}
	cfg.ApplyDefaults()
	cfg.Server.Listen = "127.0.0.1:0"
	cfg.Server.HandshakeTimeout.Duration = 30 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := NewServer(cfg, pool, core.NewSessionManager(time.Minute, time.Hour), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	time.Sleep(3 * cfg.Server.HandshakeTimeout.Duration)
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	buf := []byte{0}
	n, err := conn.Read(buf)
	if err == nil || n != 0 {
		t.Fatalf("expected slow handshake connection to be closed, n=%d err=%v", n, err)
	}
}

func TestCloseCancelsInFlightDial(t *testing.T) {
	pool := core.NewPool()
	pool.AddValidated([]core.Candidate{{Protocol: core.ProtocolSingBox, Host: "1.1.1.1", Port: 80, Source: "test"}}, nil)
	cand := pool.List()[0]
	dialer := &blockingDialer{entered: make(chan struct{}), exited: make(chan struct{})}
	pool.RegisterDialer(cand.Fingerprint, dialer)
	cfg := config.Config{}
	cfg.ApplyDefaults()
	cfg.Server.Listen = "127.0.0.1:0"
	cfg.Auth.Enabled = true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := NewServer(cfg, pool, core.NewSessionManager(time.Minute, time.Hour), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	proxyURL, _ := url.Parse("http://aio:change-me@" + srv.Addr())
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 2 * time.Second}
	done := make(chan error, 1)
	go func() {
		resp, err := client.Get("http://example.test/")
		if resp != nil {
			_ = resp.Body.Close()
		}
		done <- err
	}()
	select {
	case <-dialer.entered:
	case <-time.After(time.Second):
		t.Fatal("dialer was not entered")
	}
	_ = srv.Close()
	select {
	case <-dialer.exited:
	case <-time.After(time.Second):
		t.Fatal("in-flight dial was not canceled by server close")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("client request did not finish after dial cancellation")
	}
}

type fixedTargetDialer struct{ target string }

func (d fixedTargetDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var nd net.Dialer
	return nd.DialContext(ctx, network, d.target)
}

type failDialer struct{ err error }

func (d failDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if d.err != nil {
		return nil, d.err
	}
	return nil, errors.New("dial failed")
}

type blockingDialer struct {
	entered chan struct{}
	exited  chan struct{}
	once    sync.Once
}

func (d *blockingDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d.once.Do(func() { close(d.entered) })
	<-ctx.Done()
	close(d.exited)
	return nil, ctx.Err()
}

func mustHostPort(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

func httpViaProxy(t *testing.T, proxyAddr, user, pass, targetURL string) string {
	t.Helper()
	proxyURL, err := url.Parse("http://" + url.UserPassword(user, pass).String() + "@" + proxyAddr)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 5 * time.Second}
	resp, err := client.Get(targetURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func socksGet(proxyAddr, user, pass, target string) error {
	conn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	br := bufio.NewReader(conn)
	conn.Write([]byte{0x05, 0x01, 0x02})
	head := make([]byte, 2)
	if _, err := io.ReadFull(br, head); err != nil {
		return err
	}
	if head[1] != 0x02 {
		return fmt.Errorf("method %d", head[1])
	}
	u := []byte(user)
	p := []byte(pass)
	msg := []byte{0x01, byte(len(u))}
	msg = append(msg, u...)
	msg = append(msg, byte(len(p)))
	msg = append(msg, p...)
	conn.Write(msg)
	if _, err := io.ReadFull(br, head); err != nil {
		return err
	}
	if head[1] != 0 {
		return fmt.Errorf("auth failed")
	}
	host, portStr, _ := net.SplitHostPort(target)
	port, _ := strconv.Atoi(portStr)
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	req = append(req, []byte(host)...)
	req = append(req, byte(port>>8), byte(port))
	conn.Write(req)
	resp := make([]byte, 10)
	if _, err := io.ReadFull(br, resp); err != nil {
		return err
	}
	if resp[1] != 0 {
		return fmt.Errorf("connect code %d", resp[1])
	}
	fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", target)
	raw, err := io.ReadAll(br)
	if err != nil {
		return err
	}
	if !strings.Contains(string(raw), "ok") {
		return fmt.Errorf("bad socks response %q", raw)
	}
	return nil
}
