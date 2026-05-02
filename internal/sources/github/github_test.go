package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"runloop/internal/sources"
)

type fakeSecrets struct {
	values         map[string]string
	connectionErrs map[string]error
}

func (s fakeSecrets) Resolve(ctx context.Context, id string) (string, error) {
	return s.values[id], ctx.Err()
}

func (s fakeSecrets) ResolveProfileEnv(ctx context.Context, profile, name string) (string, error) {
	return s.values[profile+"."+name], ctx.Err()
}

func (s fakeSecrets) ResolveConnectionToken(ctx context.Context, ref string) (string, error) {
	if err := s.connectionErrs[ref]; err != nil {
		return "", err
	}
	return s.values[ref], ctx.Err()
}

type legacySecrets struct {
	values map[string]string
}

func (s legacySecrets) Resolve(ctx context.Context, id string) (string, error) {
	return s.values[id], ctx.Err()
}

func (s legacySecrets) ResolveProfileEnv(ctx context.Context, profile, name string) (string, error) {
	return s.values[profile+"."+name], ctx.Err()
}

func TestNewRequiresTokenSecret(t *testing.T) {
	_, err := New("prs", map[string]any{}, sources.BuildOptions{})
	if err == nil {
		t.Fatal("expected missing auth to fail")
	}
	if got, want := err.Error(), `github_pr source "prs" requires config.connection or config.tokenSecret`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestNewRejectsConnectionAndTokenSecret(t *testing.T) {
	_, err := New("prs", map[string]any{
		"connection":  "github.work",
		"tokenSecret": "github-token",
	}, sources.BuildOptions{Secrets: fakeSecrets{values: map[string]string{}}})
	if err == nil {
		t.Fatal("expected both auth settings to fail")
	}
	if got, want := err.Error(), `github_pr source "prs" sets both config.connection and config.tokenSecret`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestNewRejectsConnectionWithoutTokenConnectionResolver(t *testing.T) {
	_, err := New("prs", map[string]any{
		"connection": "github.work",
	}, sources.BuildOptions{Secrets: legacySecrets{values: map[string]string{}}})
	if err == nil {
		t.Fatal("expected unsupported resolver to fail")
	}
	if got, want := err.Error(), `github_pr source "prs" connection "github.work" requires secrets.TokenConnectionResolver`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestSyncExpandsMeAndNormalizesUnresolvedReviewThreads(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != "Bearer gh-test" {
			t.Fatalf("Authorization = %q", got)
		}
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"data":{"viewer":{"login":"octocat"}}}`))
		case 2:
			query, _ := req.Variables["query"].(string)
			if strings.Contains(query, "@me") || !strings.Contains(query, "assignee:octocat") {
				t.Fatalf("query was not expanded to viewer login: %q", query)
			}
			_, _ = w.Write([]byte(`{
				"data": {
					"search": {
						"pageInfo": {"hasNextPage": false, "endCursor": ""},
						"nodes": [{
							"__typename": "PullRequest",
							"id": "PR_node",
							"number": 7,
							"title": "Fix the bug",
							"url": "https://github.com/acme/widgets/pull/7",
							"headRefName": "fix-bug",
							"headRefOid": "abc123",
							"baseRefName": "main",
							"repository": {
								"nameWithOwner": "acme/widgets",
								"url": "https://github.com/acme/widgets",
								"sshUrl": "git@github.com:acme/widgets.git"
							},
							"reviewThreads": {
								"nodes": [
									{
										"id": "thread-unresolved",
										"isResolved": false,
										"comments": {"nodes": [{
											"id": "comment-1",
											"body": "Please handle the nil case.",
											"url": "https://github.com/acme/widgets/pull/7#discussion_r1",
											"createdAt": "2026-05-01T08:00:00Z",
											"author": {"login": "reviewer"}
										}]}
									},
									{
										"id": "thread-resolved",
										"isResolved": true,
										"comments": {"nodes": [{
											"id": "comment-2",
											"body": "Resolved comment.",
											"url": "https://github.com/acme/widgets/pull/7#discussion_r2",
											"createdAt": "2026-05-01T08:05:00Z",
											"author": {"login": "reviewer"}
										}]}
									}
								]
							}
						}]
					}
				}
			}`))
		default:
			t.Fatalf("unexpected request %d", calls)
		}
	}))
	defer server.Close()

	src, err := New("github-assigned-prs", map[string]any{
		"tokenSecret": "github-token",
		"query":       "is:pr is:open assignee:@me",
		"pageSize":    10,
		"graphqlURL":  server.URL,
	}, sources.BuildOptions{Secrets: fakeSecrets{values: map[string]string{"github-token": "gh-test"}}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	candidates, cursor, err := src.Sync(context.Background(), sources.Cursor{})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if cursor.IsZero() {
		t.Fatal("expected cursor to advance")
	}
	if len(candidates) != 1 {
		t.Fatalf("expected one candidate, got %d", len(candidates))
	}
	candidate := candidates[0]
	if candidate.ExternalID != "acme/widgets#7" {
		t.Fatalf("ExternalID = %q", candidate.ExternalID)
	}
	if candidate.EntityType != "github_pr_unresolved_review_threads" {
		t.Fatalf("EntityType = %q", candidate.EntityType)
	}
	repo := candidate.Normalized["repository"].(map[string]any)
	if repo["cloneURL"] != "git@github.com:acme/widgets.git" {
		t.Fatalf("cloneURL = %#v", repo["cloneURL"])
	}
	ids := candidate.Normalized["unresolvedReviewThreadIDs"].([]string)
	if len(ids) != 1 || ids[0] != "thread-unresolved" {
		t.Fatalf("unresolvedReviewThreadIDs = %#v", ids)
	}
	prompt := candidate.Normalized["reviewPromptMarkdown"].(string)
	if !strings.Contains(prompt, "Please handle the nil case.") || strings.Contains(prompt, "Resolved comment.") {
		t.Fatalf("unexpected prompt markdown:\n%s", prompt)
	}
}

func TestSyncUsesConnectionToken(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != "Bearer gh-work" {
			t.Fatalf("Authorization = %q", got)
		}
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		query, _ := req.Variables["query"].(string)
		if strings.Contains(query, "@me") {
			t.Fatalf("query should not require viewer expansion: %q", query)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"search": {
					"pageInfo": {"hasNextPage": false, "endCursor": ""},
					"nodes": []
				}
			}
		}`))
	}))
	defer server.Close()

	src, err := New("prs", map[string]any{
		"connection": "github.work",
		"query":      "is:pr is:open author:octocat",
		"graphqlURL": server.URL,
	}, sources.BuildOptions{Secrets: fakeSecrets{values: map[string]string{"github.work": "gh-work"}}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	candidates, _, err := src.Sync(context.Background(), sources.Cursor{})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected no candidates, got %d", len(candidates))
	}
	if calls != 1 {
		t.Fatalf("expected one request, got %d", calls)
	}
}

func TestTokenConnectionResolveErrorIncludesSourceAndConnection(t *testing.T) {
	src, err := New("prs", map[string]any{
		"connection": "github.work",
		"query":      "is:pr is:open author:octocat",
	}, sources.BuildOptions{Secrets: fakeSecrets{
		connectionErrs: map[string]error{"github.work": errors.New("backend unavailable")},
	}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = src.token(context.Background())
	if err == nil {
		t.Fatal("expected token resolution to fail")
	}
	if got, want := err.Error(), `github_pr source "prs" connection "github.work" token: backend unavailable`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestTokenConnectionEmptyTokenIncludesSourceAndConnection(t *testing.T) {
	src, err := New("prs", map[string]any{
		"connection": "github.work",
		"query":      "is:pr is:open author:octocat",
	}, sources.BuildOptions{Secrets: fakeSecrets{values: map[string]string{"github.work": ""}}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = src.token(context.Background())
	if err == nil {
		t.Fatal("expected empty token to fail")
	}
	if got, want := err.Error(), `github_pr source "prs" connection "github.work" resolved empty token`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
