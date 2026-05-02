package secrets

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGitHubUserDeviceProviderUnexpiredTokenReturnsExistingAccessToken(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	setGitHubProviderNow(t, now)
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		t.Fatalf("unexpected refresh request")
	}))
	t.Cleanup(server.Close)
	setGitHubTokenEndpoint(t, server.URL)
	writeGitHubTokenFile(t, dir, "github-work-oauth.json", map[string]any{
		"access_token":             "gho-existing-access",
		"refresh_token":            "ghr-existing-refresh",
		"expires_at":               now.Add(3 * time.Minute).Format(time.RFC3339),
		"refresh_token_expires_at": now.Add(24 * time.Hour).Format(time.RFC3339),
	}, 0o600)
	writeSecretsConfig(t, dir, `connections:
  github:
    work:
      provider: github_user_device
      clientID: client-id
      tokenFile: secrets/github-work-oauth.json
`)

	resolver, err := NewFileResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	value, err := resolver.ResolveConnectionToken(context.Background(), "github.work")
	if err != nil {
		t.Fatal(err)
	}
	if value != "gho-existing-access" {
		t.Fatalf("value = %q", value)
	}
	if called {
		t.Fatal("refresh endpoint was called")
	}
	if err := resolver.TestConnection(context.Background(), "github.work"); err != nil {
		t.Fatalf("test connection: %v", err)
	}
}

func TestGitHubUserDeviceProviderExpiringTokenRefreshesAndPersistsToken(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	setGitHubProviderNow(t, now)
	var gotForm url.Values
	var gotAccept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		gotForm = r.PostForm
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token":"gho-new-access",
			"refresh_token":"ghr-new-refresh",
			"expires_in":3600,
			"refresh_token_expires_in":7200
		}`))
	}))
	t.Cleanup(server.Close)
	setGitHubTokenEndpoint(t, server.URL)
	writeGitHubTokenFile(t, dir, "github-work-oauth.json", map[string]any{
		"access_token":             "gho-old-access",
		"refresh_token":            "ghr-old-refresh",
		"expires_at":               now.Add(time.Minute).Format(time.RFC3339),
		"refresh_token_expires_at": now.Add(24 * time.Hour).Format(time.RFC3339),
	}, 0o600)
	writeSecretsConfig(t, dir, `connections:
  github:
    work:
      provider: github_user_device
      clientID: client-id
      tokenFile: secrets/github-work-oauth.json
`)

	resolver, err := NewFileResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	value, err := resolver.ResolveConnectionToken(context.Background(), "github.work")
	if err != nil {
		t.Fatal(err)
	}
	if value != "gho-new-access" {
		t.Fatalf("value = %q", value)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept = %q", gotAccept)
	}
	if gotForm.Get("client_id") != "client-id" || gotForm.Get("grant_type") != "refresh_token" || gotForm.Get("refresh_token") != "ghr-old-refresh" {
		t.Fatalf("form = %v", gotForm)
	}

	path := filepath.Join(dir, "secrets", "github-work-oauth.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var persisted githubUserDeviceToken
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.AccessToken != "gho-new-access" || persisted.RefreshToken != "ghr-new-refresh" {
		t.Fatalf("persisted token = %#v", persisted)
	}
	if !persisted.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("expires_at = %s", persisted.ExpiresAt)
	}
	if !persisted.RefreshTokenExpiresAt.Equal(now.Add(2 * time.Hour)) {
		t.Fatalf("refresh_token_expires_at = %s", persisted.RefreshTokenExpiresAt)
	}
}

func TestGitHubUserDeviceProviderExpiredRefreshTokenRequiresReconnect(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	setGitHubProviderNow(t, now)
	writeGitHubTokenFile(t, dir, "github-work-oauth.json", map[string]any{
		"access_token":             "gho-old-access",
		"refresh_token":            "ghr-old-refresh",
		"expires_at":               now.Add(-time.Minute).Format(time.RFC3339),
		"refresh_token_expires_at": now.Add(-time.Second).Format(time.RFC3339),
	}, 0o600)
	writeGitHubConnectionConfig(t, dir, "secrets/github-work-oauth.json")

	resolver, err := NewFileResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.ResolveConnectionToken(context.Background(), "github.work")
	if err == nil {
		t.Fatal("expected error")
	}
	assertReconnectErrorWithoutTokenLeak(t, err, "gho-old-access", "ghr-old-refresh")
}

func TestGitHubUserDeviceProviderInvalidRefreshResponseRequiresReconnectWithoutLeakingTokens(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "bad refresh token", body: `{"error":"bad_refresh_token"}`},
		{name: "invalid grant", body: `{"error":"invalid_grant"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
			setGitHubProviderNow(t, now)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)
			setGitHubTokenEndpoint(t, server.URL)
			writeGitHubTokenFile(t, dir, "github-work-oauth.json", map[string]any{
				"access_token":             "gho-secret-access",
				"refresh_token":            "ghr-secret-refresh",
				"expires_at":               now.Add(time.Minute).Format(time.RFC3339),
				"refresh_token_expires_at": now.Add(24 * time.Hour).Format(time.RFC3339),
			}, 0o600)
			writeGitHubConnectionConfig(t, dir, "secrets/github-work-oauth.json")

			resolver, err := NewFileResolver(dir)
			if err != nil {
				t.Fatal(err)
			}
			_, err = resolver.ResolveConnectionToken(context.Background(), "github.work")
			if err == nil {
				t.Fatal("expected error")
			}
			assertReconnectErrorWithoutTokenLeak(t, err, "gho-secret-access", "ghr-secret-refresh")
		})
	}
}

func TestGitHubUserDeviceProviderUnsafeTokenFilePermissionsFail(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	setGitHubProviderNow(t, now)
	writeGitHubTokenFile(t, dir, "github-work-oauth.json", map[string]any{
		"access_token":             "gho-existing-access",
		"refresh_token":            "ghr-existing-refresh",
		"expires_at":               now.Add(3 * time.Minute).Format(time.RFC3339),
		"refresh_token_expires_at": now.Add(24 * time.Hour).Format(time.RFC3339),
	}, 0o644)
	writeGitHubConnectionConfig(t, dir, "secrets/github-work-oauth.json")

	resolver, err := NewFileResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.ResolveConnectionToken(context.Background(), "github.work")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must not be group- or world-readable") {
		t.Fatalf("error = %q", err.Error())
	}
	assertNoTokenLeak(t, err, "gho-existing-access", "ghr-existing-refresh")
}

func TestGitHubUserDeviceProviderSymlinkTokenFileFails(t *testing.T) {
	dir := t.TempDir()
	outsideDir := t.TempDir()
	now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	setGitHubProviderNow(t, now)
	writeGitHubTokenFile(t, outsideDir, "github-work-oauth.json", map[string]any{
		"access_token":             "gho-outside-access",
		"refresh_token":            "ghr-outside-refresh",
		"expires_at":               now.Add(3 * time.Minute).Format(time.RFC3339),
		"refresh_token_expires_at": now.Add(24 * time.Hour).Format(time.RFC3339),
	}, 0o600)
	secretsDir := filepath.Join(dir, "secrets")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outsideDir, "secrets", "github-work-oauth.json"), filepath.Join(secretsDir, "github-work-oauth.json")); err != nil {
		t.Fatal(err)
	}
	writeGitHubConnectionConfig(t, dir, "secrets/github-work-oauth.json")

	resolver, err := NewFileResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.ResolveConnectionToken(context.Background(), "github.work")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must not be a symlink") {
		t.Fatalf("error = %q", err.Error())
	}
	assertNoTokenLeak(t, err, "gho-outside-access", "ghr-outside-refresh")
}

func TestGitHubUserDeviceProviderUnsafeTokenFilePathsFail(t *testing.T) {
	tests := []struct {
		name      string
		tokenFile string
		want      string
	}{
		{name: "absolute", tokenFile: "/tmp/github-oauth.json", want: "must be relative to config dir"},
		{name: "escaping", tokenFile: "../github-oauth.json", want: "escapes config dir"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeGitHubConnectionConfig(t, dir, tt.tokenFile)
			resolver, err := NewFileResolver(dir)
			if err != nil {
				t.Fatal(err)
			}
			_, err = resolver.ResolveConnectionToken(context.Background(), "github.work")
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestGitHubUserDeviceProviderResolveConnectionEnvReportsProviderKind(t *testing.T) {
	dir := t.TempDir()
	writeGitHubConnectionConfig(t, dir, "secrets/github-work-oauth.json")
	resolver, err := NewFileResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.ResolveConnectionEnv(context.Background(), "github.work", "GITHUB_TOKEN")
	if err == nil {
		t.Fatal("expected error")
	}
	want := `connection "github.work" provider "github_user_device" does not resolve env "GITHUB_TOKEN"`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func writeGitHubTokenFile(t *testing.T, dir, name string, token map[string]any, perm os.FileMode) {
	t.Helper()
	data, err := json.Marshal(token)
	if err != nil {
		t.Fatal(err)
	}
	secretsDir := filepath.Join(dir, "secrets")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, name), data, perm); err != nil {
		t.Fatal(err)
	}
}

func writeGitHubConnectionConfig(t *testing.T, dir, tokenFile string) {
	t.Helper()
	writeSecretsConfig(t, dir, `connections:
  github:
    work:
      provider: github_user_device
      clientID: client-id
      tokenFile: `+tokenFile+`
`)
}

func setGitHubProviderNow(t *testing.T, now time.Time) {
	t.Helper()
	oldNow := githubProviderNow
	githubProviderNow = func() time.Time { return now }
	t.Cleanup(func() { githubProviderNow = oldNow })
}

func setGitHubTokenEndpoint(t *testing.T, endpoint string) {
	t.Helper()
	oldEndpoint := githubTokenEndpoint
	githubTokenEndpoint = endpoint
	t.Cleanup(func() { githubTokenEndpoint = oldEndpoint })
}

func assertReconnectErrorWithoutTokenLeak(t *testing.T, err error, tokens ...string) {
	t.Helper()
	if !strings.Contains(err.Error(), "must be reconnected") {
		t.Fatalf("error = %q, want reconnect-required error", err.Error())
	}
	assertNoTokenLeak(t, err, tokens...)
}

func assertNoTokenLeak(t *testing.T, err error, tokens ...string) {
	t.Helper()
	for _, token := range tokens {
		if strings.Contains(err.Error(), token) {
			t.Fatalf("error leaks token %q: %q", token, err.Error())
		}
	}
}
