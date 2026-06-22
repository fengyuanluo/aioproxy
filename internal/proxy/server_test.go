package proxy

import (
	"bufio"
	"context"
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

type fixedTargetDialer struct{ target string }

func (d fixedTargetDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var nd net.Dialer
	return nd.DialContext(ctx, network, d.target)
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
