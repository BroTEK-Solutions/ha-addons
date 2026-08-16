package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func newHandler(optionsPath, metricsURL string, browser bool) http.Handler {
	client := &http.Client{Timeout: 5 * time.Second}
	dialer := &net.Dialer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(collectStatus(r.Context(), optionsPath, metricsURL, browser, client, dialer.DialContext, time.Now))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = fmt.Fprint(w, page)
	})
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; frame-ancestors 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func ingressOnly(sourceIP string, next http.Handler) http.Handler {
	expected := net.ParseIP(sourceIP)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || expected == nil || !expected.Equal(net.ParseIP(host)) {
			http.Error(w, "ingress access required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("grafana-sm-ui: ")
	browser := envOr("REPORTER_APP", "grafana_sm") == "grafana_sm_browser"
	server := &http.Server{Addr: envOr("LISTEN_ADDR", "0.0.0.0:4051"), Handler: ingressOnly(envOr("INGRESS_SOURCE_IP", "172.30.32.2"), newHandler(envOr("OPTIONS_FILE", "/data/options.json"), envOr("AGENT_METRICS_URL", "http://127.0.0.1:4050/metrics"), browser)), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("status page listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
