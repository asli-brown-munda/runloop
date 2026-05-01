package gitcheckout

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"runloop/internal/steps"
	"runloop/internal/workflows"
)

type fakeGitRunner struct {
	calls [][]string
}

func (r *fakeGitRunner) Run(ctx context.Context, args []string, dir string) (Output, error) {
	r.calls = append(r.calls, append([]string{}, args...))
	if len(args) >= 4 && args[0] == "-C" && args[2] == "rev-parse" && args[3] == "HEAD" {
		return Output{Stdout: "abc123\n"}, ctx.Err()
	}
	return Output{}, ctx.Err()
}

func TestGitCheckoutFetchesPullRefIntoWorkspace(t *testing.T) {
	runner := &fakeGitRunner{}
	oldRunner := commandRunner
	commandRunner = runner
	defer func() { commandRunner = oldRunner }()

	workspace := t.TempDir()
	_, result := Execute(context.Background(), steps.Request{
		Step:     workflows.Step{ID: "checkout", Type: "git_checkout"},
		Workflow: workflows.Workflow{Permissions: workflows.Permissions{Shell: true}},
		Input: map[string]any{
			"repoURL":    "git@github.com:acme/widgets.git",
			"pullNumber": 7,
			"headSHA":    "abc123",
		},
		StepCtx: map[string]any{"runloop": map[string]any{"workspace": workspace}},
	})
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	wantPath := filepath.Join(workspace, "repo")
	if result.Data["path"] != wantPath {
		t.Fatalf("path = %#v, want %q", result.Data["path"], wantPath)
	}
	if result.Data["headSHA"] != "abc123" {
		t.Fatalf("headSHA = %#v", result.Data["headSHA"])
	}
	wantCalls := [][]string{
		{"init", wantPath},
		{"-C", wantPath, "remote", "add", "origin", "git@github.com:acme/widgets.git"},
		{"-C", wantPath, "fetch", "--depth=1", "origin", "refs/pull/7/head"},
		{"-C", wantPath, "checkout", "--detach", "FETCH_HEAD"},
		{"-C", wantPath, "rev-parse", "HEAD"},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("git calls = %#v\nwant %#v", runner.calls, wantCalls)
	}
}

func TestGitCheckoutRejectsHeadSHAMismatch(t *testing.T) {
	runner := &fakeGitRunner{}
	oldRunner := commandRunner
	commandRunner = runner
	defer func() { commandRunner = oldRunner }()

	_, result := Execute(context.Background(), steps.Request{
		Step:     workflows.Step{ID: "checkout", Type: "git_checkout"},
		Workflow: workflows.Workflow{Permissions: workflows.Permissions{Shell: true}},
		Input: map[string]any{
			"repoURL":    "git@github.com:acme/widgets.git",
			"pullNumber": 7,
			"headSHA":    "def456",
		},
		StepCtx: map[string]any{"runloop": map[string]any{"workspace": t.TempDir()}},
	})
	if result.OK {
		t.Fatalf("expected mismatch to fail, result = %#v", result)
	}
	if result.Error == nil || result.Error.Message != `checked out HEAD "abc123" did not match expected "def456"` {
		t.Fatalf("error = %#v", result.Error)
	}
}

type failingGitRunner struct{}

func (failingGitRunner) Run(context.Context, []string, string) (Output, error) {
	return Output{ExitCode: 1, Stderr: "git failed"}, errors.New("exit status 1")
}

func TestGitCheckoutRequiresShellPermission(t *testing.T) {
	_, result := Execute(context.Background(), steps.Request{
		Step:     workflows.Step{ID: "checkout", Type: "git_checkout"},
		Workflow: workflows.Workflow{},
		Input:    map[string]any{"repoURL": "git@github.com:acme/widgets.git", "pullNumber": 7},
		StepCtx:  map[string]any{"runloop": map[string]any{"workspace": t.TempDir()}},
	})
	if result.OK {
		t.Fatal("expected missing shell permission to fail")
	}
}

func TestGitCheckoutReportsGitFailure(t *testing.T) {
	oldRunner := commandRunner
	commandRunner = failingGitRunner{}
	defer func() { commandRunner = oldRunner }()

	workspace := t.TempDir()
	_, result := Execute(context.Background(), steps.Request{
		Step:     workflows.Step{ID: "checkout", Type: "git_checkout"},
		Workflow: workflows.Workflow{Permissions: workflows.Permissions{Shell: true}},
		Input:    map[string]any{"repoURL": "git@github.com:acme/widgets.git", "pullNumber": 7},
		StepCtx:  map[string]any{"runloop": map[string]any{"workspace": workspace}},
	})

	if result.OK {
		t.Fatalf("expected git failure, result = %#v", result)
	}
	if result.Error == nil || result.Error.Message == "" {
		t.Fatalf("error = %#v", result.Error)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].Path != "git.log" {
		t.Fatalf("artifacts = %#v", result.Artifacts)
	}
}
