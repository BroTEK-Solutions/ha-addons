package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSupervisorClientReadsSavesAndRestartsSelf(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer supervisor-token" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/addons/self/info":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "ok", "data": map[string]any{"options": map[string]any{"operation_mode": "local"}}})
		case "/addons/self/options", "/addons/self/restart":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newSupervisorClient(server.URL, "supervisor-token", server.Client())
	options, err := client.Options(context.Background())
	if err != nil || options["operation_mode"] != "local" {
		t.Fatalf("Options() = %#v, %v", options, err)
	}
	candidate := map[string]any{"operation_mode": "fleet"}
	if err := client.Save(context.Background(), candidate); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := client.Restart(context.Background()); err != nil {
		t.Fatalf("Restart() error = %v", err)
	}
	want := []string{
		"GET /addons/self/info",
		"POST /addons/self/options",
		"POST /addons/self/restart",
	}
	if !reflectStrings(paths, want) {
		t.Fatalf("requests = %#v, want %#v", paths, want)
	}
}

func reflectStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
