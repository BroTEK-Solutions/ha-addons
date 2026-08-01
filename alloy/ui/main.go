package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

type supervisorAPI interface {
	Restart(context.Context) error
}

type apiResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type saveRequest struct {
	Options map[string]any     `json:"options"`
	Secrets map[string]*string `json:"secrets"`
}

type statusResponse struct {
	AlloyReady     bool `json:"alloy_ready"`
	SafeMode       bool `json:"safe_mode"`
	ManualOverride bool `json:"manual_override"`
}

func newAppHandler(store settingsStore, validator candidateValidator, supervisor supervisorAPI, alloyURL string) (http.Handler, error) {
	proxy, err := newAlloyProxy(alloyURL)
	if err != nil {
		return nil, err
	}
	assets, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/alloy/", proxy)
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		settings, err := store.Load()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		manualOverride, _ := settings["manual_config_enabled"].(bool)
		writeJSON(w, http.StatusOK, statusResponse{
			AlloyReady:     alloyReady(r.Context(), alloyURL),
			SafeMode:       envBool("SAFE_MODE"),
			ManualOverride: manualOverride,
		})
	})
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			options, err := store.Load()
			if err != nil {
				writeError(w, http.StatusBadGateway, err)
				return
			}
			writeJSON(w, http.StatusOK, projectConfig(options))
		case http.MethodPost:
			if r.Header.Get("Content-Type") != "application/json" {
				writeError(w, http.StatusUnsupportedMediaType, errors.New("Content-Type must be application/json"))
				return
			}
			var input saveRequest
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&input); err != nil {
				writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
				return
			}
			mode, _ := input.Options["operation_mode"].(string)
			manualOverride, _ := input.Options["manual_config_enabled"].(bool)
			if !manualOverride && mode != "fleet" && mode != "local" {
				writeError(w, http.StatusBadRequest, errors.New("choose Fleet Management or Local configuration before saving"))
				return
			}
			current, err := store.Load()
			if err != nil {
				writeError(w, http.StatusBadGateway, err)
				return
			}
			candidate := mergeOptions(current, input.Options, input.Secrets)
			if err := validateModeRequirements(candidate); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if err := validator.Validate(r.Context(), candidate); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if err := store.Save(candidate); err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Message: "Configuration saved. Restart Alloy to apply it."})
		default:
			w.Header().Set("Allow", "GET, POST")
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		}
	})
	mux.HandleFunc("/api/restart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		if err := supervisor.Restart(r.Context()); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusAccepted, apiResponse{OK: true, Message: "Restart requested."})
	})
	mux.Handle("/", http.FileServer(http.FS(assets)))
	return securityHeaders(mux), nil
}

func alloyReady(ctx context.Context, alloyURL string) bool {
	probeContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(probeContext, http.MethodGet, strings.TrimRight(alloyURL, "/")+"/-/ready", nil)
	if err != nil {
		return false
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode >= 200 && response.StatusCode < 300
}

func envBool(key string) bool {
	value, err := strconv.ParseBool(os.Getenv(key))
	return err == nil && value
}

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

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; frame-ancestors 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, apiResponse{OK: false, Message: err.Error()})
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--migrate-only" {
		store := newFileStore(
			envOr("SETTINGS_FILE", "/data/settings.json"),
			envOr("LEGACY_OPTIONS_FILE", "/data/options.json"),
		)
		if _, err := store.Load(); err != nil {
			log.Fatal(err)
		}
		return
	}
	token := os.Getenv("SUPERVISOR_TOKEN")
	if token == "" {
		log.Fatal("SUPERVISOR_TOKEN is required")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	supervisor := newSupervisorClient(envOr("SUPERVISOR_URL", "http://supervisor"), token, client)
	store := newFileStore(envOr("SETTINGS_FILE", "/data/settings.json"), envOr("LEGACY_OPTIONS_FILE", "/data/options.json"))
	validator := newAlloyValidator(envOr("GENERATOR", "/usr/share/alloy/generate-config.sh"), envOr("ALLOY_BIN", "/usr/bin/alloy"))
	handler, err := newAppHandler(store, validator, supervisor, envOr("ALLOY_URL", "http://127.0.0.1:12345"))
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{
		Addr:              envOr("LISTEN_ADDR", "0.0.0.0:8099"),
		Handler:           ingressOnly(envOr("INGRESS_SOURCE_IP", "172.30.32.2"), handler),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if level := envOr("UI_LOG_LEVEL", "info"); level == "debug" || level == "info" {
		log.Printf("Alloy configuration UI listening on %s", server.Addr)
	}
	log.Fatal(server.ListenAndServe())
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
