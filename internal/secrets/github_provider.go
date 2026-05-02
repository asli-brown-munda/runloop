package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

var (
	githubTokenEndpoint = "https://github.com/login/oauth/access_token"
	githubProviderNow   = time.Now
)

type githubUserDeviceToken struct {
	AccessToken           string    `json:"access_token"`
	RefreshToken          string    `json:"refresh_token"`
	ExpiresAt             time.Time `json:"expires_at"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

func (r *FileResolver) resolveGitHubUserDeviceToken(ctx context.Context, ref string, entry connectionEntry) (string, error) {
	if entry.ClientID == "" {
		return "", fmt.Errorf("connection %q clientID is required", ref)
	}
	if entry.TokenFile == "" {
		return "", fmt.Errorf("connection %q tokenFile is required", ref)
	}
	path, err := r.resolveSecretPath(entry.TokenFile)
	if err != nil {
		return "", fmt.Errorf("connection %q tokenFile: %w", ref, err)
	}

	r.githubRefreshMu.Lock()
	defer r.githubRefreshMu.Unlock()

	token, err := r.readGitHubUserDeviceToken(path)
	if err != nil {
		return "", fmt.Errorf("connection %q tokenFile: %w", ref, err)
	}
	now := githubProviderNow().UTC()
	if token.ExpiresAt.After(now.Add(2 * time.Minute)) {
		return token.AccessToken, nil
	}
	if !token.RefreshTokenExpiresAt.IsZero() && !token.RefreshTokenExpiresAt.After(now) {
		return "", githubReconnectRequiredError(ref, "refresh token is expired")
	}
	refreshed, err := refreshGitHubUserDeviceToken(ctx, entry.ClientID, token, now)
	if err != nil {
		return "", fmt.Errorf("connection %q: %w", ref, err)
	}
	if err := writeGitHubUserDeviceToken(path, refreshed); err != nil {
		return "", fmt.Errorf("connection %q tokenFile: %w", ref, err)
	}
	return refreshed.AccessToken, nil
}

func (r *FileResolver) readGitHubUserDeviceToken(path string) (githubUserDeviceToken, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return githubUserDeviceToken{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return githubUserDeviceToken{}, fmt.Errorf("file must not be a symlink")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return githubUserDeviceToken{}, fmt.Errorf("file must not be group- or world-readable")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return githubUserDeviceToken{}, err
	}
	var token githubUserDeviceToken
	if err := json.Unmarshal(data, &token); err != nil {
		return githubUserDeviceToken{}, err
	}
	if token.AccessToken == "" {
		return githubUserDeviceToken{}, fmt.Errorf("access_token is required")
	}
	if token.RefreshToken == "" {
		return githubUserDeviceToken{}, fmt.Errorf("refresh_token is required")
	}
	if token.ExpiresAt.IsZero() {
		return githubUserDeviceToken{}, fmt.Errorf("expires_at is required")
	}
	return token, nil
}

type githubRefreshResponse struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresIn             int64  `json:"expires_in"`
	RefreshTokenExpiresIn int64  `json:"refresh_token_expires_in"`
	Error                 string `json:"error"`
}

func refreshGitHubUserDeviceToken(ctx context.Context, clientID string, current githubUserDeviceToken, now time.Time) (githubUserDeviceToken, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", current.RefreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenEndpoint, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return githubUserDeviceToken{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return githubUserDeviceToken{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return githubUserDeviceToken{}, err
	}
	var parsed githubRefreshResponse
	if len(body) > 0 {
		_ = json.Unmarshal(body, &parsed)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return githubUserDeviceToken{}, githubReconnectRequiredErrorForRefresh(parsed.Error)
	}
	if parsed.Error != "" {
		return githubUserDeviceToken{}, githubReconnectRequiredErrorForRefresh(parsed.Error)
	}
	if parsed.AccessToken == "" || parsed.ExpiresIn <= 0 {
		return githubUserDeviceToken{}, githubReconnectRequiredErrorForRefresh("invalid_response")
	}

	refreshed := current
	refreshed.AccessToken = parsed.AccessToken
	refreshed.ExpiresAt = now.Add(time.Duration(parsed.ExpiresIn) * time.Second).UTC()
	if parsed.RefreshToken != "" {
		refreshed.RefreshToken = parsed.RefreshToken
	}
	if parsed.RefreshTokenExpiresIn > 0 {
		refreshed.RefreshTokenExpiresAt = now.Add(time.Duration(parsed.RefreshTokenExpiresIn) * time.Second).UTC()
	}
	return refreshed, nil
}

func writeGitHubUserDeviceToken(path string, token githubUserDeviceToken) error {
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".github-token-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTmp := true
	defer func() {
		if removeTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	removeTmp = false
	return nil
}

func githubReconnectRequiredError(ref, reason string) error {
	return fmt.Errorf("connection %q GitHub credentials must be reconnected: %s", ref, reason)
}

func githubReconnectRequiredErrorForRefresh(code string) error {
	switch code {
	case "bad_refresh_token", "invalid_grant":
		return fmt.Errorf("GitHub credentials must be reconnected: refresh token was rejected")
	default:
		return fmt.Errorf("GitHub credentials must be reconnected: refresh failed")
	}
}
