package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxAlloyHTML = 4 << 20

func newAlloyProxy(rawURL string) (http.Handler, error) {
	return newAlloyProxyWithBudgets(rawURL, 10*time.Second, 30*time.Second)
}

func newAlloyProxyWithBudgets(rawURL string, headerBudget, idleBudget time.Duration) (http.Handler, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = headerBudget
	// Alloy is a local HTTP/1 server. Connection-level idle deadlines must not
	// be shared by concurrent HTTP/2 streams.
	transport.ForceAttemptHTTP2 = false
	dialer := &net.Dialer{Timeout: headerBudget, KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &idleProxyConn{Conn: conn, budget: idleBudget}, nil
	}
	proxy.Transport = transport
	originalDirector := proxy.Director
	proxy.Director = func(request *http.Request) {
		originalDirector(request)
		// HTML has to be rewritten for Home Assistant's tokenized ingress base.
		request.Header.Set("Accept-Encoding", "identity")
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		if location := response.Header.Get("Location"); strings.HasPrefix(location, "/alloy/") {
			response.Header.Set("Location", "./"+strings.TrimPrefix(location, "/alloy/"))
		}
		if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/html") {
			return nil
		}
		// HTML is buffered for rewriting, not streamed. Bound its total read
		// time as well as its size even if the upstream keeps dribbling bytes.
		upstreamBody := response.Body
		timer := time.AfterFunc(idleBudget, func() { _ = upstreamBody.Close() })
		defer timer.Stop()
		body, err := io.ReadAll(io.LimitReader(response.Body, maxAlloyHTML+1))
		if err != nil {
			return fmt.Errorf("read Alloy HTML: %w", err)
		}
		_ = response.Body.Close()
		if len(body) > maxAlloyHTML {
			return fmt.Errorf("Alloy HTML exceeds %d bytes", maxAlloyHTML)
		}
		body = bytes.ReplaceAll(body, []byte(`href="/alloy/"`), []byte(`href="./"`))
		response.Body = io.NopCloser(bytes.NewReader(body))
		response.ContentLength = int64(len(body))
		response.Header.Set("Content-Length", strconv.Itoa(len(body)))
		response.Header.Del("ETag")
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "Alloy is not ready", http.StatusBadGateway)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A lifetime WriteTimeout would truncate healthy long-lived streams.
		// Apply a fresh write deadline only while writing/flushing each chunk.
		_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
		proxy.ServeHTTP(&idleProxyWriter{ResponseWriter: w, budget: idleBudget}, r)
	}), nil
}

type idleProxyConn struct {
	net.Conn
	budget time.Duration
}

func (c *idleProxyConn) Read(p []byte) (int, error) {
	if err := c.Conn.SetReadDeadline(time.Now().Add(c.budget)); err != nil {
		return 0, err
	}
	return c.Conn.Read(p)
}

func (c *idleProxyConn) Write(p []byte) (int, error) {
	if err := c.Conn.SetWriteDeadline(time.Now().Add(c.budget)); err != nil {
		return 0, err
	}
	return c.Conn.Write(p)
}

type idleProxyWriter struct {
	http.ResponseWriter
	budget time.Duration
}

func (w *idleProxyWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *idleProxyWriter) Write(p []byte) (int, error) {
	controller := http.NewResponseController(w.ResponseWriter)
	_ = controller.SetWriteDeadline(time.Now().Add(w.budget))
	defer controller.SetWriteDeadline(time.Time{}) //nolint:errcheck -- the next write establishes its own deadline
	return w.ResponseWriter.Write(p)
}

func (w *idleProxyWriter) FlushError() error {
	controller := http.NewResponseController(w.ResponseWriter)
	_ = controller.SetWriteDeadline(time.Now().Add(w.budget))
	defer controller.SetWriteDeadline(time.Time{}) //nolint:errcheck -- the next write establishes its own deadline
	return controller.Flush()
}
