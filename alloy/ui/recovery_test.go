package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSafeModeRuntimeIgnoresSavedSettings(t *testing.T) {
	for _, settings := range []string{`{"alloy_additional_args":"--bogus","alloy_disable_telemetry":false,"loki_password":"stored-sentinel"}`, `{"broken":`} {
		dir := t.TempDir()
		path := filepath.Join(dir, "settings.json")
		if err := os.WriteFile(path, []byte(settings), 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		command := exec.CommandContext(ctx, "bash", "-c", `
bashio::config.true() { [ "$1" = safe_mode ]; }
bashio::config() { printf '%s' generally-available; }
bashio::log.info() { :; }
exec() { printf '%s\n' "$@"; printf 'password=%s\n' "${LOKI_PASSWORD-}"; exit 0; }
export -f bashio::config.true bashio::config bashio::log.info exec
bash "$1"
`, "test", "../rootfs/etc/s6-overlay/s6-rc.d/alloy/run")
		command.Env = append(os.Environ(), "SETTINGS_FILE="+path, "LOKI_PASSWORD=inherited-sentinel")
		output, err := command.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("runtime failed: %v %s", err, output)
		}
		for _, bad := range []string{"--bogus", "stored-sentinel", "inherited-sentinel", "parse error"} {
			if strings.Contains(string(output), bad) {
				t.Errorf("safe mode consumed saved state (%s): %s", bad, output)
			}
		}
		if !strings.Contains(string(output), "--disable-reporting") {
			t.Errorf("safe mode did not use known-good flags: %s", output)
		}
	}
}

func TestRecoveryRequiresSafeModeIngressAndExplicitConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	damaged := []byte(`{"broken":`)
	if err := os.WriteFile(path, damaged, 0600); err != nil {
		t.Fatal(err)
	}
	store := newFileStore(path, path+".legacy")
	handler, err := newAppHandler(store, fakeValidator{}, &fakeSupervisor{}, "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	protected := ingressOnly("172.30.32.2", handler)
	request := func(remote, contentType, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/recover", strings.NewReader(body))
		r.RemoteAddr = remote
		r.Header.Set("Content-Type", contentType)
		response := httptest.NewRecorder()
		protected.ServeHTTP(response, r)
		return response
	}
	t.Setenv("SAFE_MODE", "false")
	if got := request("172.30.32.2:1234", "application/json", `{"confirm":true}`).Code; got != http.StatusForbidden {
		t.Fatalf("recovery outside safe mode = %d", got)
	}
	t.Setenv("SAFE_MODE", "true")
	for _, test := range []struct {
		remote, contentType, body string
		code                      int
	}{
		{"192.0.2.1:1234", "application/json", `{"confirm":true}`, http.StatusForbidden},
		{"172.30.32.2:1234", "text/plain", `{"confirm":true}`, http.StatusUnsupportedMediaType},
		{"172.30.32.2:1234", "application/json", `{"confirm":false}`, http.StatusBadRequest},
		{"172.30.32.2:1234", "application/json", `{"confirm":true} {}`, http.StatusBadRequest},
	} {
		if got := request(test.remote, test.contentType, test.body).Code; got != test.code {
			t.Fatalf("recovery status=%d want=%d", got, test.code)
		}
	}
	response := request("172.30.32.2:1234", "application/json", `{"confirm":true}`)
	if response.Code != http.StatusOK {
		t.Fatalf("recovery = %d %s", response.Code, response.Body.String())
	}
	archives, err := filepath.Glob(filepath.Join(filepath.Dir(path), "backups", "*.json"))
	if err != nil || len(archives) != 1 {
		t.Fatalf("archives=%v err=%v", archives, err)
	}
	contents, err := os.ReadFile(archives[0])
	if err != nil || string(contents) != string(damaged) {
		t.Fatalf("original bytes not preserved: %q %v", contents, err)
	}
	info, err := os.Stat(archives[0])
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("archive not owner-only: %v %v", info, err)
	}
	settings, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if settings["restart_required"] != true || len(settings) != 2 {
		t.Fatalf("reset state=%v", settings)
	}
}

func TestRecoveryStatusWorksWithUnreadableSettings(t *testing.T) {
	t.Setenv("SAFE_MODE", "true")
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	handler, err := newAppHandler(newFileStore(path, path+".legacy"), fakeValidator{}, &fakeSupervisor{}, "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	var status statusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || !status.RecoveryAvailable || !status.SafeMode || status.SettingsReadable {
		t.Fatalf("recovery status=%d %#v", response.Code, status)
	}
}

func TestSettingsLockCancellationPreservesOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store := newFileStore(path, path+".legacy")
	if err := store.Save(map[string]any{"instance_name": "old"}); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(path+".lock", os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	ran := false
	err = store.Update(ctx, func(settings map[string]any) (map[string]any, error) {
		ran = true
		settings["instance_name"] = "new"
		return settings, nil
	})
	if err == nil || ran {
		t.Fatalf("cancelled lock wait update ran=%v err=%v", ran, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `"old"`) {
		t.Fatalf("original changed: %s %v", data, err)
	}
}

func TestRecoveryArchiveFailurePreservesOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	original := []byte("unreadable original")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	// A file at the archive directory path makes archive creation fail without
	// relying on permission errors (which root/container tests can bypass).
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "backups"), []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	store := newFileStore(path, path+".legacy")
	if err := store.Recover(context.Background()); err == nil {
		t.Fatal("archive failure was ignored")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(original) {
		t.Fatalf("failed recovery changed original: %s %v", after, err)
	}
}
