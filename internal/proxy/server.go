package proxy

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aioproxy/aioproxy/internal/config"
	"github.com/aioproxy/aioproxy/internal/core"
)

const dialAttemptTimeout = 30 * time.Second

type Server struct {
	cfg          config.Config
	pool         *core.Pool
	sessions     *core.SessionManager
	logger       *slog.Logger
	listener     net.Listener
	ctx          context.Context
	cancel       context.CancelFunc
	shuttingDown atomic.Bool
	wg           sync.WaitGroup
	connMu       sync.Mutex
	conns        map[net.Conn]struct{}
}

func NewServer(cfg config.Config, pool *core.Pool, sessions *core.SessionManager, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, pool: pool, sessions: sessions, logger: logger, conns: map[net.Conn]struct{}{}}
}

func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Server.Listen)
	if err != nil {
		return err
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.listener = ln
	s.logger.Info("proxy listener started", "listen", s.cfg.Server.Listen)
	go func() { <-s.ctx.Done(); _ = s.Close() }()
	go s.acceptLoop()
	return nil
}

func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) Close() error {
	s.shuttingDown.Store(true)
	if s.cancel != nil {
		s.cancel()
	}
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}
func (s *Server) trackConn(conn net.Conn) {
	s.connMu.Lock()
	s.conns[conn] = struct{}{}
	s.connMu.Unlock()
}

func (s *Server) untrackConn(conn net.Conn) {
	s.connMu.Lock()
	delete(s.conns, conn)
	s.connMu.Unlock()
}

func (s *Server) forceCloseActive() {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	for conn := range s.conns {
		_ = conn.Close()
	}
}

func (s *Server) Wait(timeout time.Duration) {
	ch := make(chan struct{})
	go func() { s.wg.Wait(); close(ch) }()
	select {
	case <-ch:
		return
	case <-time.After(timeout):
		s.forceCloseActive()
	}
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
	}
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.shuttingDown.Load() {
				return
			}
			s.logger.Warn("proxy accept failed", "error", err)
			continue
		}
		s.trackConn(conn)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer s.untrackConn(conn)
			s.handleConn(conn)
		}()
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	if timeout := s.cfg.Server.HandshakeTimeout.Duration; timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
	}
	br := bufio.NewReader(conn)
	b, err := br.Peek(1)
	if err != nil {
		return
	}
	if b[0] == 0x05 {
		s.handleSOCKS(conn, br)
		return
	}
	s.handleHTTPConn(conn, br)
}

func (s *Server) handleSOCKS(client net.Conn, br *bufio.Reader) {
	ver, err := br.ReadByte()
	if err != nil || ver != 0x05 {
		return
	}
	nm, err := br.ReadByte()
	if err != nil {
		return
	}
	methods := make([]byte, int(nm))
	if _, err := io.ReadFull(br, methods); err != nil {
		return
	}
	method := byte(0x00)
	if s.cfg.Auth.Enabled {
		method = 0x02
	}
	if !containsByte(methods, method) {
		client.Write([]byte{0x05, 0xff})
		return
	}
	client.Write([]byte{0x05, method})
	info := core.SessionInfo{}
	if s.cfg.Auth.Enabled {
		u, p, ok := readSocksAuth(br, client)
		if !ok {
			return
		}
		parsed, authOK := s.authenticate(u, p)
		if !authOK {
			client.Write([]byte{0x01, 0x01})
			return
		}
		client.Write([]byte{0x01, 0x00})
		info = parsed
	}
	req := make([]byte, 4)
	if _, err := io.ReadFull(br, req); err != nil {
		return
	}
	if req[0] != 0x05 || req[1] != 0x01 {
		client.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	host, err := readSocksAddr(br, req[3])
	if err != nil {
		client.Write([]byte{0x05, 0x04, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	portb := make([]byte, 2)
	if _, err := io.ReadFull(br, portb); err != nil {
		return
	}
	port := int(portb[0])<<8 | int(portb[1])
	target := net.JoinHostPort(host, strconv.Itoa(port))
	_ = client.SetReadDeadline(time.Time{})
	up, cand, err := s.dialScheduled(target, info)
	if err != nil {
		client.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer up.Close()
	client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	s.tunnel(client, up, cand)
}

func (s *Server) handleHTTPConn(client net.Conn, br *bufio.Reader) {
	for {
		if !s.handleHTTP(client, br) {
			return
		}
	}
}

func (s *Server) handleHTTP(client net.Conn, br *bufio.Reader) bool {
	req, err := http.ReadRequest(br)
	if err != nil {
		return false
	}
	clientClose := req.Close
	info := core.SessionInfo{}
	if s.cfg.Auth.Enabled {
		u, p, ok := parseProxyAuth(req.Header.Get("Proxy-Authorization"))
		if !ok {
			writeHTTPProxyAuthRequired(client)
			return false
		}
		parsed, authOK := s.authenticate(u, p)
		if !authOK {
			writeHTTPProxyAuthRequired(client)
			return false
		}
		info = parsed
	}
	_ = client.SetReadDeadline(time.Time{})
	req.Header.Del("Proxy-Authorization")
	if req.Method == http.MethodConnect {
		target := req.Host
		if !strings.Contains(target, ":") {
			target = net.JoinHostPort(target, "443")
		}
		up, cand, err := s.dialScheduled(target, info)
		if err != nil {
			writeHTTPError(client, http.StatusServiceUnavailable, err.Error())
			return false
		}
		defer up.Close()
		io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n")
		s.tunnel(client, up, cand)
		return false
	}
	target, err := httpTarget(req)
	if err != nil {
		writeHTTPError(client, http.StatusBadRequest, err.Error())
		return false
	}
	up, cand, err := s.dialScheduled(target, info)
	if err != nil {
		writeHTTPError(client, http.StatusServiceUnavailable, err.Error())
		return false
	}
	defer up.Close()
	if req.URL.IsAbs() {
		req.URL.Scheme = ""
		req.URL.Host = ""
		req.RequestURI = ""
	}
	req.Close = true
	req.Header.Set("Connection", "close")
	if err := req.Write(up); err != nil {
		s.pool.MarkFailure(cand.Fingerprint, err.Error(), s.cfg.RuntimeFailure.MaxFailures)
		return false
	}
	resp, err := http.ReadResponse(bufio.NewReader(up), req)
	if err != nil {
		s.pool.MarkFailure(cand.Fingerprint, err.Error(), s.cfg.RuntimeFailure.MaxFailures)
		return false
	}
	defer resp.Body.Close()
	resp.Close = false
	resp.Header.Del("Connection")
	if err := resp.Write(client); err != nil {
		s.pool.MarkFailure(cand.Fingerprint, err.Error(), s.cfg.RuntimeFailure.MaxFailures)
		return false
	}
	s.pool.MarkSuccess(cand.Fingerprint)
	return !clientClose
}

func (s *Server) authenticate(username, password string) (core.SessionInfo, bool) {
	info, ok := core.ParseSessionUsername(username, s.cfg.Auth.Username, s.cfg.Session.DefaultTTL.Duration, s.cfg.Session.MaxTTL.Duration)
	return info, ok && password == s.cfg.Auth.Password
}

func (s *Server) dialScheduled(target string, info core.SessionInfo) (net.Conn, core.Candidate, error) {
	attempted := map[string]struct{}{}
	maxAttempts := s.cfg.RuntimeFailure.RetryAttempts + 1
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	var failures []string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		cand, ok := s.pickCandidateForAttempt(info, attempted)
		if !ok {
			if len(failures) == 0 {
				return nil, core.Candidate{}, errors.New("candidate pool is empty")
			}
			return nil, core.Candidate{}, fmt.Errorf("no more route-matching candidates after %d attempt(s): %s", len(failures), failures[len(failures)-1])
		}
		attempted[cand.Fingerprint] = struct{}{}
		parent := s.ctx
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithTimeout(parent, dialAttemptTimeout)
		conn, err := DialViaCandidate(ctx, cand, target, s.pool.Dialer(cand.Fingerprint))
		cancel()
		if err == nil {
			if info.SessionID != "" {
				s.sessions.Rebind(info, cand.Fingerprint)
			}
			if attempt > 1 {
				s.logger.Info("proxy candidate retry succeeded", "session", info.SessionID, "plugin", info.Plugin, "region", info.Region, "attempt", attempt, "candidate", cand.Fingerprint[:16], "source", cand.Source)
			}
			return conn, cand, nil
		}
		s.pool.MarkFailure(cand.Fingerprint, err.Error(), s.cfg.RuntimeFailure.MaxFailures)
		if info.SessionID != "" {
			s.sessions.Unbind(info)
		}
		failures = append(failures, err.Error())
		s.logger.Warn("proxy candidate attempt failed", "session", info.SessionID, "plugin", info.Plugin, "region", info.Region, "attempt", attempt, "candidate", cand.Fingerprint[:16], "source", cand.Source, "error", err)
	}
	return nil, core.Candidate{}, fmt.Errorf("all %d route-matching candidate attempt(s) failed: %s", len(failures), failures[len(failures)-1])
}

func (s *Server) pickCandidateForAttempt(info core.SessionInfo, attempted map[string]struct{}) (core.Candidate, bool) {
	match := func(c core.Candidate) bool { return c.MatchesRoute(info.Plugin, info.Region) }
	if info.SessionID != "" {
		if bound, ok := s.sessions.GetBound(info, s.pool); ok {
			if _, seen := attempted[bound.Fingerprint]; !seen {
				return bound, true
			}
		}
	}
	return s.pool.PickMatchingExcluding(s.cfg.Scheduler.Policy, match, attempted)
}

func (s *Server) tunnel(a net.Conn, b net.Conn, cand core.Candidate) {
	var bytes atomic.Int64
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(&countingWriter{Writer: b, count: &bytes}, a)
		_ = b.SetDeadline(time.Now())
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(&countingWriter{Writer: a, count: &bytes}, b)
		_ = a.SetDeadline(time.Now())
		done <- struct{}{}
	}()
	earlyClosed := false
	window := s.cfg.RuntimeFailure.EarlyFailureWindow.Duration
	if window <= 0 {
		<-done
	} else {
		timer := time.NewTimer(window)
		select {
		case <-done:
			earlyClosed = true
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
		}
	}
	if earlyClosed && bytes.Load() == 0 {
		s.pool.MarkFailure(cand.Fingerprint, "early zero-byte closure", s.cfg.RuntimeFailure.MaxFailures)
	} else if bytes.Load() > 0 {
		s.pool.MarkSuccess(cand.Fingerprint)
	}
	<-done
	if bytes.Load() > 0 {
		s.pool.MarkSuccess(cand.Fingerprint)
	}
}

type countingWriter struct {
	io.Writer
	count *atomic.Int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	w.count.Add(int64(n))
	return n, err
}

func readSocksAuth(br *bufio.Reader, client net.Conn) (string, string, bool) {
	head := make([]byte, 2)
	if _, err := io.ReadFull(br, head); err != nil || head[0] != 0x01 {
		return "", "", false
	}
	u := make([]byte, int(head[1]))
	if _, err := io.ReadFull(br, u); err != nil {
		return "", "", false
	}
	plen, err := br.ReadByte()
	if err != nil {
		return "", "", false
	}
	p := make([]byte, int(plen))
	if _, err := io.ReadFull(br, p); err != nil {
		return "", "", false
	}
	return string(u), string(p), true
}
func readSocksAddr(br *bufio.Reader, atyp byte) (string, error) {
	switch atyp {
	case 0x01:
		b := make([]byte, 4)
		_, err := io.ReadFull(br, b)
		return net.IP(b).String(), err
	case 0x04:
		b := make([]byte, 16)
		_, err := io.ReadFull(br, b)
		return net.IP(b).String(), err
	case 0x03:
		l, err := br.ReadByte()
		if err != nil {
			return "", err
		}
		b := make([]byte, int(l))
		_, err = io.ReadFull(br, b)
		return string(b), err
	}
	return "", fmt.Errorf("invalid atyp")
}
func containsByte(bs []byte, b byte) bool {
	for _, x := range bs {
		if x == b {
			return true
		}
	}
	return false
}
func parseProxyAuth(h string) (string, string, bool) {
	fields := strings.Fields(h)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Basic") {
		return "", "", false
	}
	dec, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(dec), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
func writeHTTPProxyAuthRequired(w io.Writer) {
	io.WriteString(w, "HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"AIOPROXY\"\r\nContent-Length: 0\r\n\r\n")
}
func writeHTTPError(w io.Writer, code int, msg string) {
	body := http.StatusText(code)
	if msg != "" {
		body = msg
	}
	fmt.Fprintf(w, "HTTP/1.1 %d %s\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", code, http.StatusText(code), len(body), body)
}
func httpTarget(req *http.Request) (string, error) {
	host := req.URL.Host
	scheme := req.URL.Scheme
	if host == "" {
		host = req.Host
	}
	if host == "" {
		return "", fmt.Errorf("missing host")
	}
	if !strings.Contains(host, ":") {
		port := "80"
		if scheme == "https" {
			port = "443"
		}
		host = net.JoinHostPort(host, port)
	}
	return host, nil
}
