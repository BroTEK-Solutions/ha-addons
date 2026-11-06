package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Use the production server configuration: a headers-only timeout cannot stop
// this request, because its complete headers precede the stalled JSON body.
func TestServerBoundsStalledConfigBody(t *testing.T) {
	handler, err := newAppHandler(&fakeStore{}, fakeValidator{}, &fakeSupervisor{}, "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Config = newHTTPServer("", handler)
	server.Start()
	defer server.Close()
	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(12 * time.Second))
	_, err = fmt.Fprintf(conn, "POST /api/config HTTP/1.1\r\nHost: test\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n{")
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("stalled upload was not rejected before client deadline: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 400 {
		t.Fatalf("stalled upload status = %d", response.StatusCode)
	}
}

// A shell's child inherits its output pipe. Killing only the shell leaves both
// the child and CombinedOutput's pipe reader alive until the child exits.
func TestValidationCancellationStopsShellChildren(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "alloy")
	completed := filepath.Join(dir, "child-completed")
	body := "#!/bin/sh\n(sleep 2; echo completed > \"$CHILD_COMPLETED\") &\nwait\n"
	if err := os.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CHILD_COMPLETED", completed)
	validator := newAlloyValidator("unused", script, "")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	began := time.Now()
	err := validator.Validate(ctx, map[string]any{"manual_config_enabled": true, "manual_config": "logging {}"})
	elapsed := time.Since(began)
	if err == nil {
		t.Fatal("cancelled validation succeeded")
	}
	if elapsed > time.Second {
		t.Errorf("cancelled validation waited %s for child", elapsed)
	}
	// Wait for the fixture's bounded side effect to prove that the subprocess
	// itself was stopped, rather than merely abandoning its output pipe.
	timer := time.NewTimer(time.Until(began.Add(2300 * time.Millisecond)))
	defer timer.Stop()
	<-timer.C
	if _, err := os.Stat(completed); !os.IsNotExist(err) {
		t.Errorf("child survived cancellation: %v", err)
	}
}

func TestServerAcceptsManualSaveAtRequestSizeLimit(t *testing.T) {
	prefix := `{"options":{"manual_config_enabled":true,"manual_config":"// `
	suffix := `"},"secrets":{}}`
	config := strings.Repeat("x", (1<<20)-len(prefix)-len(suffix))
	binary := filepath.Join(t.TempDir(), "alloy")
	// Run the actual candidate writer and command path; only the Alloy binary
	// is replaced with a fixture that verifies the complete file reached it.
	script := fmt.Sprintf("#!/bin/sh\n[ \"$(wc -c < \"$3\")\" -eq %d ]\n", len(config)+4)
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{settings: map[string]any{}}
	handler, err := newAppHandler(store, newAlloyValidator("unused", binary, ""), &fakeSupervisor{}, "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Config = newHTTPServer("", handler)
	server.Start()
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Post(server.URL+"/api/config", "application/json", strings.NewReader(prefix+config+suffix))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("save status = %d: %s", response.StatusCode, body)
	}
	if got := store.saved["manual_config"]; got != "// "+config {
		t.Fatal("manual configuration was truncated")
	}
}

// A server write timeout does not cancel handler work. The API budget must
// reach the validator and prevent persistence once that budget expires.
func TestAPIBudgetCancelsValidationBeforeSaving(t *testing.T) {
	store := &fakeStore{settings: map[string]any{}}
	validator := contextValidator{started: make(chan struct{})}
	handler, err := newAppHandler(store, validator, &fakeSupervisor{}, "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(withAPIBudget(handler, 100*time.Millisecond))
	defer server.Close()
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Post(server.URL+"/api/config", "application/json", strings.NewReader(`{"options":{"manual_config_enabled":true,"manual_config":"logging {}"}}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("validation timeout status = %d", response.StatusCode)
	}
	if store.saves != 0 {
		t.Fatalf("expired candidate saved %d times", store.saves)
	}
	select {
	case <-validator.started:
	default:
		t.Fatal("validation was never started")
	}
}

type contextValidator struct{ started chan struct{} }

func (v contextValidator) Validate(ctx context.Context, _ map[string]any) error {
	close(v.started)
	<-ctx.Done()
	// A subprocess may exit successfully just as its deadline fires. The
	// handler must check its context even when the validator returns success.
	return nil
}
