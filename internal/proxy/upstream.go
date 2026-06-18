package proxy

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aioproxy/aioproxy/internal/core"
)

type DirectDialer struct{}

func (DirectDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, address)
}

func DialViaCandidate(ctx context.Context, candidate core.Candidate, target string, custom core.CandidateDialer) (net.Conn, error) {
	if custom != nil {
		return custom.DialContext(ctx, "tcp", target)
	}
	switch candidate.Protocol {
	case core.ProtocolHTTP:
		return dialHTTPProxy(ctx, candidate, target)
	case core.ProtocolSOCKS5:
		return dialSOCKS5Proxy(ctx, candidate, target)
	default:
		return nil, fmt.Errorf("unsupported candidate protocol %s", candidate.Protocol)
	}
}

func dialHTTPProxy(ctx context.Context, c core.Candidate, target string) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", c.Address())
	if err != nil {
		return nil, err
	}
	deadline, _ := ctx.Deadline()
	if !deadline.IsZero() {
		_ = conn.SetDeadline(deadline)
	}
	var b strings.Builder
	b.WriteString("CONNECT " + target + " HTTP/1.1\r\n")
	b.WriteString("Host: " + target + "\r\n")
	if c.Username != "" || c.Password != "" {
		token := base64.StdEncoding.EncodeToString([]byte(c.Username + ":" + c.Password))
		b.WriteString("Proxy-Authorization: Basic " + token + "\r\n")
	}
	b.WriteString("\r\n")
	if _, err := io.WriteString(conn, b.String()); err != nil {
		conn.Close()
		return nil, err
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		conn.Close()
		return nil, err
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		conn.Close()
		return nil, fmt.Errorf("http proxy CONNECT status %d", resp.StatusCode)
	}
	_ = conn.SetDeadline(time.Time{})
	if br.Buffered() > 0 {
		return &bufferedConn{Conn: conn, r: br}, nil
	}
	return conn, nil
}

func dialSOCKS5Proxy(ctx context.Context, c core.Candidate, target string) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", c.Address())
	if err != nil {
		return nil, err
	}
	deadline, _ := ctx.Deadline()
	if !deadline.IsZero() {
		_ = conn.SetDeadline(deadline)
	}
	methods := []byte{0x00}
	if c.Username != "" || c.Password != "" {
		methods = append(methods, 0x02)
	}
	if _, err := conn.Write(append([]byte{0x05, byte(len(methods))}, methods...)); err != nil {
		conn.Close()
		return nil, err
	}
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		conn.Close()
		return nil, err
	}
	if buf[0] != 0x05 {
		conn.Close()
		return nil, fmt.Errorf("invalid socks version")
	}
	if buf[1] == 0x02 {
		u := []byte(c.Username)
		p := []byte(c.Password)
		if len(u) > 255 || len(p) > 255 {
			conn.Close()
			return nil, fmt.Errorf("socks credential too long")
		}
		msg := []byte{0x01, byte(len(u))}
		msg = append(msg, u...)
		msg = append(msg, byte(len(p)))
		msg = append(msg, p...)
		if _, err := conn.Write(msg); err != nil {
			conn.Close()
			return nil, err
		}
		if _, err := io.ReadFull(conn, buf); err != nil {
			conn.Close()
			return nil, err
		}
		if buf[1] != 0x00 {
			conn.Close()
			return nil, fmt.Errorf("socks auth failed")
		}
	} else if buf[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("socks no acceptable auth method")
	}
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		conn.Close()
		return nil, err
	}
	port, _ := strconv.Atoi(portStr)
	req := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			req = append(req, 0x01)
			req = append(req, v4...)
		} else {
			req = append(req, 0x04)
			req = append(req, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			conn.Close()
			return nil, fmt.Errorf("target host too long")
		}
		req = append(req, 0x03, byte(len(host)))
		req = append(req, []byte(host)...)
	}
	req = append(req, byte(port>>8), byte(port))
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		conn.Close()
		return nil, err
	}
	if buf[0] != 0x05 || buf[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("socks connect failed code=%d", buf[1])
	}
	// RSV + ATYP
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		conn.Close()
		return nil, err
	}
	atyp := buf[1]
	var skip int
	switch atyp {
	case 0x01:
		skip = 4
	case 0x04:
		skip = 16
	case 0x03:
		one := []byte{0}
		if _, err := io.ReadFull(conn, one); err != nil {
			conn.Close()
			return nil, err
		}
		skip = int(one[0])
	default:
		conn.Close()
		return nil, fmt.Errorf("invalid socks atyp")
	}
	if skip > 0 {
		if _, err := io.CopyN(io.Discard, conn, int64(skip)); err != nil {
			conn.Close()
			return nil, err
		}
	}
	if _, err := io.CopyN(io.Discard, conn, 2); err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func ParseHTTPProxyURL(raw string, source string) (core.Candidate, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return core.Candidate{}, false
	}
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		if u.Scheme == "https" {
			port = 443
		} else {
			port = 80
		}
	}
	c := core.Candidate{Protocol: core.ProtocolHTTP, Host: u.Hostname(), Port: port, Source: source, Metadata: map[string]string{"scheme": u.Scheme}}
	if u.User != nil {
		c.Username = u.User.Username()
		c.Password, _ = u.User.Password()
	}
	c.Normalize()
	return c, true
}

func ParseSOCKSProxyURL(raw string, source string) (core.Candidate, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return core.Candidate{}, false
	}
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 1080
	}
	c := core.Candidate{Protocol: core.ProtocolSOCKS5, Host: u.Hostname(), Port: port, Source: source, Metadata: map[string]string{"scheme": u.Scheme}}
	if u.User != nil {
		c.Username = u.User.Username()
		c.Password, _ = u.User.Password()
	}
	c.Normalize()
	return c, true
}

type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }
