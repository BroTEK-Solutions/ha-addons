package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStatusAPIReportsUnavailableOptions(t *testing.T) {
	for _, body := range []string{"missing", `{broken`, `null`, `{} trailing`, `{}`} {
		t.Run(body, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "missing.json")
			if body != "missing" {
				path = writeOptions(t, body)
			}
			handler, err := newHandler(path, "http://127.0.0.1:1")
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(handler)
			defer server.Close()
			resp, err := server.Client().Get(server.URL + "/api/status")
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatal(err)
			}
			want := body == `{}`
			if got, ok := result["options_available"].(bool); !ok || got != want {
				t.Fatalf("options_available=%v, want %v", result["options_available"], want)
			}
		})
	}
}

// A real HTTP connection with the production ten-second write deadline catches
// the original sequential outage path: its response arrived after the socket's
// deadline and the browser received EOF instead of useful partial diagnostics.
func TestStatusHTTPCompletesDuringMultipleEndpointOutages(t *testing.T) {
	var active, peak atomic.Int32
	dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(4 * time.Second):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
		}
	}))
	defer agent.Close()
	entries := make([]string, 12)
	for i := range entries {
		entries[i] = fmt.Sprintf("endpoint-%02d.invalid:443", i)
	}
	encoded, _ := json.Marshal(map[string]any{"allowed_endpoints": entries})
	optionsPath := writeOptions(t, string(encoded))
	handler, err := newHandlerWithDial(optionsPath, agent.URL, dial)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Config.WriteTimeout = 10 * time.Second
	server.Start()
	defer server.Close()
	defer server.CloseClientConnections()
	client := &http.Client{Timeout: 8 * time.Second}
	started := time.Now()
	resp, err := client.Get(server.URL + "/api/status")
	if err != nil {
		t.Fatalf("status must arrive before the HTTP write deadline: %v", err)
	}
	defer resp.Body.Close()
	var result statusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 7*time.Second {
		t.Fatalf("whole-request budget exceeded: %s", elapsed)
	}
	if len(result.Endpoints) != len(entries) {
		t.Fatalf("lost endpoint results: %d", len(result.Endpoints))
	}
	if peak.Load() > 4 || peak.Load() < 2 {
		t.Fatalf("expected bounded concurrent probes, peak=%d", peak.Load())
	}
	if active.Load() != 0 {
		t.Fatalf("probes still running after response: %d", active.Load())
	}
	if !result.AgentResponding {
		t.Fatal("metrics probe must run concurrently and preserve its success")
	}
	untested := 0
	for _, entry := range result.Endpoints {
		if strings.Contains(entry.Detail, "not tested") {
			untested++
		}
		if entry.Reachable || entry.Detail == "" {
			t.Fatalf("outage must have an explanation: %+v", entry)
		}
	}
	if untested == 0 {
		t.Fatal("budget-exhausted entries must be marked not tested")
	}
}

func TestStatusHonorsCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := collectStatus(ctx, writeOptions(t, `{"allowed_endpoints":["database.invalid:443"]}`), "http://127.0.0.1:1", http.DefaultClient, func(context.Context, string, string) (net.Conn, error) {
		t.Error("cancelled request must not dial")
		return nil, errors.New("unexpected dial")
	}, time.Now)
	if len(result.Endpoints) != 1 || result.Endpoints[0].Reachable {
		t.Fatalf("cancelled diagnostic should stay unknown: %+v", result)
	}
}
