package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
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
