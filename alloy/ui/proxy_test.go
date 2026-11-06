package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAlloyProxyMakesHTMLBaseIngressRelative(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<html><head><base href="/alloy/" /></head></html>`)
	}))
	defer upstream.Close()

	proxy, err := newAlloyProxy(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/alloy/", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, request)

	want := `<html><head><base href="./" /></head></html>`
	if response.Code != http.StatusOK || response.Body.String() != want {
		t.Fatalf("proxy response = %d %q, want %q", response.Code, response.Body.String(), want)
	}
	if response.Header().Get("Content-Encoding") != "" {
		t.Fatalf("rewritten HTML is still encoded: %q", response.Header().Get("Content-Encoding"))
	}
}

func TestAlloyProxyPreservesPrefixedPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.URL.Path)
	}))
	defer upstream.Close()

	proxy, err := newAlloyProxy(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/alloy/graph", nil)
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "/alloy/graph" {
		t.Fatalf("proxy response = %d %q", response.Code, response.Body.String())
	}
}

func TestAlloyProxyBoundsUnresponsiveUpstream(t *testing.T) {
	for _, mode := range []string{"headers", "html", "stream"} {
		t.Run(mode, func(t *testing.T) {
			stopped := make(chan struct{})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(stopped)
				if mode != "headers" {
					contentType := "text/html"
					if mode == "stream" {
						contentType = "text/event-stream"
					}
					w.Header().Set("Content-Type", contentType)
					_, _ = io.WriteString(w, "initial")
					w.(http.Flusher).Flush()
				}
				<-r.Context().Done()
			}))
			defer upstream.Close()
			proxy, err := newAlloyProxyWithBudgets(upstream.URL, 100*time.Millisecond, 200*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(proxy)
			defer server.Close()
			client := &http.Client{Timeout: time.Second}
			began := time.Now()
			response, err := client.Get(server.URL + "/alloy/")
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			_, readErr := io.ReadAll(response.Body)
			if elapsed := time.Since(began); elapsed > 750*time.Millisecond {
				t.Fatalf("stalled %s lasted %s; only the client timeout released it", mode, elapsed)
			}
			if mode == "stream" {
				if readErr == nil {
					t.Fatal("stalled stream was reported as complete")
				}
			} else if response.StatusCode != http.StatusBadGateway {
				t.Fatalf("stalled %s status = %d", mode, response.StatusCode)
			}
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("upstream request survived timeout")
			}
		})
	}
}

func TestAlloyProxyPreservesActiveStreamBeyondServerWriteTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 6; i++ {
			_, _ = io.WriteString(w, "data: ready\n\n")
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(60 * time.Millisecond):
			}
		}
	}))
	defer upstream.Close()
	proxy, err := newAlloyProxyWithBudgets(upstream.URL, 100*time.Millisecond, 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(proxy)
	server.Config = newHTTPServer("", proxy)
	server.Config.WriteTimeout = 100 * time.Millisecond
	server.Start()
	defer server.Close()
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(server.URL + "/alloy/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("active stream truncated: %v", err)
	}
	if string(body) != strings.Repeat("data: ready\n\n", 6) {
		t.Fatalf("stream = %q", body)
	}
}

func TestAlloyProxyBoundsBlockedClientWrites(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		chunk := strings.Repeat("x", 64<<10)
		for i := 0; i < 4096; i++ {
			if _, err := io.WriteString(w, chunk); err != nil {
				return
			}
		}
	}))
	defer upstream.Close()
	proxy, err := newAlloyProxyWithBudgets(upstream.URL, 100*time.Millisecond, 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		proxy.ServeHTTP(w, r)
	}))
	defer server.Close()
	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.(*net.TCPConn).SetReadBuffer(1024); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, err = fmt.Fprintf(conn, "GET /alloy/ HTTP/1.1\r\nHost: test\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d", response.StatusCode)
	}
	// Deliberately leave the body unread. A request-level write deadline was
	// cleared for streaming, so the per-write budget must release this handler.
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("proxy remained blocked on client write")
	}
}
