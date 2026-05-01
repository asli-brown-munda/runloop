package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"runloop/internal/sources"
)

const (
	Type                  = "github_pr"
	defaultGraphQLURL     = "https://api.github.com/graphql"
	defaultQueryTemplate  = "is:pr is:open assignee:@me"
	defaultEvery          = 5 * time.Minute
	defaultPageSize       = 50
	maxPageSize           = 100
	EntityUnresolvedPR    = "github_pr_unresolved_review_threads"
	EntityReviewCleanPR   = "github_pr_review_clean"
	graphQLContentType    = "application/json"
	authorizationTemplate = "Bearer %s"
)

func init() {
	sources.Register(Type, func(id string, cfg map[string]any, opts sources.BuildOptions) (sources.Source, error) {
		return New(id, cfg, opts)
	})
}

type Source struct {
	id          string
	tokenSecret string
	query       string
	every       time.Duration
	pageSize    int
	graphQLURL  string
	secrets     tokenResolver
	client      *http.Client
	now         func() time.Time
}

type tokenResolver interface {
	Resolve(context.Context, string) (string, error)
}

func New(id string, cfg map[string]any, opts sources.BuildOptions) (*Source, error) {
	tokenSecret, _ := cfg["tokenSecret"].(string)
	if tokenSecret == "" {
		return nil, fmt.Errorf("github_pr source %q requires config.tokenSecret", id)
	}
	if opts.Secrets == nil {
		return nil, fmt.Errorf("github_pr source %q requires a secrets resolver", id)
	}
	query, _ := cfg["query"].(string)
	if query == "" {
		query = defaultQueryTemplate
	}
	every := defaultEvery
	if raw, _ := cfg["every"].(string); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("github_pr source %q invalid every %q: %w", id, raw, err)
		}
		if parsed <= 0 {
			return nil, fmt.Errorf("github_pr source %q every must be positive", id)
		}
		every = parsed
	}
	pageSize := intFromConfig(cfg["pageSize"], defaultPageSize)
	if pageSize <= 0 || pageSize > maxPageSize {
		return nil, fmt.Errorf("github_pr source %q pageSize must be between 1 and %d", id, maxPageSize)
	}
	graphQLURL, _ := cfg["graphqlURL"].(string)
	if graphQLURL == "" {
		graphQLURL = defaultGraphQLURL
	}
	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &Source{
		id:          id,
		tokenSecret: tokenSecret,
		query:       query,
		every:       every,
		pageSize:    pageSize,
		graphQLURL:  graphQLURL,
		secrets:     opts.Secrets,
		client:      client,
		now:         time.Now,
	}, nil
}

func (s *Source) ID() string   { return s.id }
func (s *Source) Type() string { return Type }

func (s *Source) Test(ctx context.Context) error {
	token, err := s.token(ctx)
	if err != nil {
		return err
	}
	_, err = s.viewerLogin(ctx, token)
	return err
}

func (s *Source) WaitForChange(ctx context.Context) error {
	timer := time.NewTimer(s.every)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *Source) Sync(ctx context.Context, cursor sources.Cursor) ([]sources.InboxCandidate, sources.Cursor, error) {
	if err := ctx.Err(); err != nil {
		return nil, cursor, err
	}
	token, err := s.token(ctx)
	if err != nil {
		return nil, cursor, err
	}
	query := s.query
	if strings.Contains(query, "@me") {
		login, err := s.viewerLogin(ctx, token)
		if err != nil {
			return nil, cursor, err
		}
		query = strings.ReplaceAll(query, "@me", login)
	}
	prs, err := s.searchPullRequests(ctx, token, query)
	if err != nil {
		return nil, cursor, err
	}
	candidates := make([]sources.InboxCandidate, 0, len(prs))
	for _, pr := range prs {
		candidates = append(candidates, s.candidate(pr))
	}
	return candidates, sources.Cursor{Value: s.now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *Source) token(ctx context.Context) (string, error) {
	token, err := s.secrets.Resolve(ctx, s.tokenSecret)
	if err != nil {
		return "", fmt.Errorf("github_pr source %q token: %w", s.id, err)
	}
	if token == "" {
		return "", fmt.Errorf("github_pr source %q token secret %q resolved empty", s.id, s.tokenSecret)
	}
	return token, nil
}

func (s *Source) viewerLogin(ctx context.Context, token string) (string, error) {
	var out viewerResponse
	if err := s.graphQL(ctx, token, `query ViewerLogin { viewer { login } }`, nil, &out); err != nil {
		return "", err
	}
	if out.Data.Viewer.Login == "" {
		return "", fmt.Errorf("github_pr source %q viewer login was empty", s.id)
	}
	return out.Data.Viewer.Login, nil
}

func (s *Source) searchPullRequests(ctx context.Context, token, query string) ([]pullRequestNode, error) {
	var all []pullRequestNode
	var after *string
	for {
		var out searchResponse
		variables := map[string]any{"query": query, "first": s.pageSize, "after": after}
		if err := s.graphQL(ctx, token, searchQuery, variables, &out); err != nil {
			return nil, err
		}
		for _, node := range out.Data.Search.Nodes {
			if node.TypeName == "PullRequest" {
				all = append(all, node)
			}
		}
		if !out.Data.Search.PageInfo.HasNextPage {
			break
		}
		if out.Data.Search.PageInfo.EndCursor == "" {
			return nil, fmt.Errorf("github_pr source %q search pageInfo missing endCursor", s.id)
		}
		cursor := out.Data.Search.PageInfo.EndCursor
		after = &cursor
	}
	return all, nil
}

func (s *Source) graphQL(ctx context.Context, token, query string, variables map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.graphQLURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf(authorizationTemplate, token))
	req.Header.Set("Content-Type", graphQLContentType)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("github_pr source %q graphql request: %w", s.id, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("github_pr source %q graphql status %d", s.id, resp.StatusCode)
	}
	var envelope struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("github_pr source %q decode graphql response: %w", s.id, err)
	}
	data, err := json.Marshal(out)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Errors) > 0 {
		return fmt.Errorf("github_pr source %q graphql error: %s", s.id, envelope.Errors[0].Message)
	}
	return nil
}

func (s *Source) candidate(pr pullRequestNode) sources.InboxCandidate {
	unresolved := unresolvedThreads(pr.ReviewThreads.Nodes)
	entityType := EntityReviewCleanPR
	if len(unresolved) > 0 {
		entityType = EntityUnresolvedPR
	}
	externalID := fmt.Sprintf("%s#%d", pr.Repository.NameWithOwner, pr.Number)
	observedAt := s.now().UTC()
	if pr.UpdatedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, pr.UpdatedAt); err == nil {
			observedAt = parsed.UTC()
		}
	}
	ids := make([]string, 0, len(unresolved))
	threadPayloads := make([]map[string]any, 0, len(unresolved))
	for _, thread := range unresolved {
		ids = append(ids, thread.ID)
		threadPayloads = append(threadPayloads, threadPayload(thread))
	}
	normalized := map[string]any{
		"sourceId":                  s.id,
		"id":                        pr.ID,
		"number":                    pr.Number,
		"title":                     pr.Title,
		"url":                       pr.URL,
		"author":                    actorLogin(pr.Author),
		"baseRefName":               pr.BaseRefName,
		"headRefName":               pr.HeadRefName,
		"headSHA":                   pr.HeadRefOID,
		"pullRef":                   fmt.Sprintf("refs/pull/%d/head", pr.Number),
		"repository":                repositoryPayload(pr.Repository),
		"unresolvedReviewThreadIDs": ids,
		"unresolvedReviewThreads":   threadPayloads,
		"reviewPromptMarkdown":      reviewPromptMarkdown(pr, unresolved),
	}
	return sources.InboxCandidate{
		SourceID:   s.id,
		ExternalID: externalID,
		EntityType: entityType,
		Title:      fmt.Sprintf("%s#%d %s", pr.Repository.NameWithOwner, pr.Number, pr.Title),
		RawPayload: normalized,
		Normalized: normalized,
		ObservedAt: observedAt,
	}
}

func unresolvedThreads(threads []reviewThreadNode) []reviewThreadNode {
	out := make([]reviewThreadNode, 0, len(threads))
	for _, thread := range threads {
		if !thread.IsResolved {
			out = append(out, thread)
		}
	}
	return out
}

func repositoryPayload(repo repositoryNode) map[string]any {
	cloneURL := repo.SSHURL
	if cloneURL == "" && repo.URL != "" {
		cloneURL = strings.TrimSuffix(repo.URL, "/") + ".git"
	}
	return map[string]any{
		"nameWithOwner": repo.NameWithOwner,
		"url":           repo.URL,
		"sshURL":        repo.SSHURL,
		"cloneURL":      cloneURL,
	}
}

func threadPayload(thread reviewThreadNode) map[string]any {
	comments := make([]map[string]any, 0, len(thread.Comments.Nodes))
	for _, comment := range thread.Comments.Nodes {
		comments = append(comments, map[string]any{
			"id":        comment.ID,
			"body":      comment.Body,
			"url":       comment.URL,
			"createdAt": comment.CreatedAt,
			"author":    actorLogin(comment.Author),
		})
	}
	return map[string]any{"id": thread.ID, "isResolved": thread.IsResolved, "comments": comments}
}

func reviewPromptMarkdown(pr pullRequestNode, threads []reviewThreadNode) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# PR %s#%d: %s\n\n", pr.Repository.NameWithOwner, pr.Number, pr.Title)
	fmt.Fprintf(&b, "URL: %s\n", pr.URL)
	fmt.Fprintf(&b, "Head: %s (%s)\n\n", pr.HeadRefName, pr.HeadRefOID)
	fmt.Fprintf(&b, "Address these unresolved GitHub review threads locally. Do not push, comment, or commit unless explicitly asked.\n\n")
	for i, thread := range threads {
		fmt.Fprintf(&b, "## Thread %d: %s\n\n", i+1, thread.ID)
		for _, comment := range thread.Comments.Nodes {
			author := actorLogin(comment.Author)
			if author == "" {
				author = "unknown"
			}
			fmt.Fprintf(&b, "- %s at %s\n\n%s\n\n", author, comment.URL, comment.Body)
		}
	}
	return b.String()
}

func actorLogin(actor actorNode) string {
	return actor.Login
}

func intFromConfig(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return fallback
	}
}

type viewerResponse struct {
	Data struct {
		Viewer struct {
			Login string `json:"login"`
		} `json:"viewer"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type searchResponse struct {
	Data struct {
		Search struct {
			PageInfo pageInfo          `json:"pageInfo"`
			Nodes    []pullRequestNode `json:"nodes"`
		} `json:"search"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type pullRequestNode struct {
	TypeName      string            `json:"__typename"`
	ID            string            `json:"id"`
	Number        int               `json:"number"`
	Title         string            `json:"title"`
	URL           string            `json:"url"`
	UpdatedAt     string            `json:"updatedAt"`
	HeadRefName   string            `json:"headRefName"`
	HeadRefOID    string            `json:"headRefOid"`
	BaseRefName   string            `json:"baseRefName"`
	Author        actorNode         `json:"author"`
	Repository    repositoryNode    `json:"repository"`
	ReviewThreads reviewThreadNodes `json:"reviewThreads"`
}

type actorNode struct {
	Login string `json:"login"`
}

type repositoryNode struct {
	NameWithOwner string `json:"nameWithOwner"`
	URL           string `json:"url"`
	SSHURL        string `json:"sshUrl"`
}

type reviewThreadNodes struct {
	Nodes []reviewThreadNode `json:"nodes"`
}

type reviewThreadNode struct {
	ID         string       `json:"id"`
	IsResolved bool         `json:"isResolved"`
	Comments   commentNodes `json:"comments"`
}

type commentNodes struct {
	Nodes []commentNode `json:"nodes"`
}

type commentNode struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	URL       string    `json:"url"`
	CreatedAt string    `json:"createdAt"`
	Author    actorNode `json:"author"`
}

const searchQuery = `
query RunloopPullRequestSearch($query: String!, $first: Int!, $after: String) {
  search(query: $query, type: ISSUE, first: $first, after: $after) {
    pageInfo {
      hasNextPage
      endCursor
    }
    nodes {
      ... on PullRequest {
        __typename
        id
        number
        title
        url
        updatedAt
        headRefName
        headRefOid
        baseRefName
        author {
          login
        }
        repository {
          nameWithOwner
          url
          sshUrl
        }
        reviewThreads(first: 100) {
          nodes {
            id
            isResolved
            comments(first: 50) {
              nodes {
                id
                body
                url
                createdAt
                author {
                  login
                }
              }
            }
          }
        }
      }
    }
  }
}`
