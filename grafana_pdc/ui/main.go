// Command grafana-pdc-ui serves a read-only status page for the Grafana PDC App
// through Home Assistant ingress.
//
// It is deliberately read-only. PDC's configuration lives in Supervisor options
// where the Home Assistant UI already validates it against the App schema, and
// duplicating that here would create a second place for the same setting to be
// wrong. What was missing was not another way to configure the connector, but
// any way at all to see whether it is working and whether the destinations in
// the allowlist can actually be reached from this App.
package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func newHandler(optionsPath, metricsURL string) (http.Handler, error) {
	assets, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	dialer := &net.Dialer{}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		status := collectStatus(r.Context(), optionsPath, metricsURL, client, dialer.DialContext, time.Now)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(status)
	})

	files := http.FileServer(http.FS(assets))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	}))
	return securityHeaders(mux), nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; style-src 'self'; script-src 'self'; frame-ancestors 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// ingressOnly rejects anything that did not arrive through the Supervisor's
// ingress proxy. The status payload names internal hosts and ports, so it is
// not something to serve to the App network at large.
func ingressOnly(sourceIP string, next http.Handler) http.Handler {
	expected := net.ParseIP(sourceIP)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", "GET")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
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
	log.SetPrefix("grafana-pdc-ui: ")
	handler, err := newHandler(
		envOr("OPTIONS_FILE", "/data/options.json"),
		envOr("PDC_METRICS_URL", "http://127.0.0.1:8090/metrics"),
	)
	if err != nil {
		log.Fatal(err)
	}
	// ReadHeaderTimeout alone does not bound a request whose declared body never
	// arrives: Go drains the unread body before finishing the response, so the
	// connection is held for as long as the client wants. IdleTimeout only
	// covers the gap between requests. ReadTimeout and WriteTimeout are what
	// actually cap a single exchange.
	server := &http.Server{
		Addr:              envOr("LISTEN_ADDR", "0.0.0.0:8099"),
		Handler:           ingressOnly(envOr("INGRESS_SOURCE_IP", "172.30.32.2"), handler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("PDC status page listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
