package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"runloop/internal/config"
)

func TestDaemonUsesConfiguredRuntimePaths(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")

	paths, err := config.DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WriteInitial(paths); err != nil {
		t.Fatal(err)
	}

	stateDir := filepath.Join(t.TempDir(), "state")
	artifactDir := filepath.Join(t.TempDir(), "artifacts")
	logDir := filepath.Join(t.TempDir(), "logs")
	port := freePort(t)
	cfg := fmt.Sprintf(`daemon:
  bindAddress: 127.0.0.1
  port: %d
  stateDir: %s
  artifactDir: %s
  logDir: %s
sources:
  file: %s
workflows:
  dir: %s
models: {}
`, port, stateDir, artifactDir, logDir, paths.SourcesFile, paths.WorkflowsDir)
	if err := os.WriteFile(paths.ConfigFile, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := New(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	errc := make(chan error, 1)
	go func() {
		errc <- d.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-errc:
			if err != nil {
				t.Fatalf("daemon Run: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("daemon did not stop")
		}
	})

	client := &http.Client{Timeout: time.Second}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHealth(t, client, baseURL)
	token := strings.TrimSpace(readTestFile(t, paths.AuthToken))
	body := []byte(`{"source":"manual","externalId":"custom-paths","title":"Custom Paths","payload":{"message":"hello"}}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/inbox", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/inbox status=%d body=%s", res.StatusCode, data)
	}

	if _, err := os.Stat(filepath.Join(stateDir, "runloop.db")); err != nil {
		t.Fatalf("expected database in configured state dir: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(artifactDir, "runs", "run_*", "sinks", "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one sink artifact under %s, got %#v", artifactDir, matches)
	}
	if log := readTestFile(t, filepath.Join(logDir, "runloopd.log")); !strings.Contains(log, "starting local API") {
		t.Fatalf("expected daemon log file to contain startup log, got %q", log)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitForHealth(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		res, err := client.Get(baseURL + "/api/health")
		if err == nil {
			var out map[string]bool
			data, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK && json.Unmarshal(data, &out) == nil && out["ok"] {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("daemon health endpoint did not become ready")
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
