package gitcheckout

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"runloop/internal/steps"
)

func init() { steps.Register("git_checkout", Execute) }

type Output struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Runner interface {
	Run(ctx context.Context, args []string, dir string) (Output, error)
}

var commandRunner Runner = execRunner{}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, args []string, dir string) (Output, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := Output{Stdout: stdout.String(), Stderr: stderr.String()}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			out.ExitCode = exitErr.ExitCode()
		} else {
			out.ExitCode = -1
		}
		return out, err
	}
	return out, nil
}

func Execute(ctx context.Context, req steps.Request) (map[string]any, steps.StepResult) {
	if !req.Workflow.Permissions.Shell {
		return req.Input, steps.StepResult{OK: false, Error: &steps.StepError{Message: "git_checkout steps are disabled unless workflow permissions.shell is true"}}
	}
	timeout := 2 * time.Minute
	if req.Step.Timeout != "" {
		parsed, err := time.ParseDuration(req.Step.Timeout)
		if err != nil {
			return req.Input, steps.StepResult{OK: false, Error: &steps.StepError{Message: err.Error()}}
		}
		timeout = parsed
	}
	spec, err := checkoutSpec(req)
	if err != nil {
		return req.Input, steps.StepResult{OK: false, Error: &steps.StepError{Message: err.Error()}}
	}
	if err := os.MkdirAll(spec.destination, 0o755); err != nil {
		return req.Input, steps.StepResult{OK: false, Error: &steps.StepError{Message: err.Error()}}
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var log strings.Builder
	for _, args := range [][]string{
		{"init", spec.destination},
		{"-C", spec.destination, "remote", "add", "origin", spec.repoURL},
		{"-C", spec.destination, "fetch", "--depth=1", "origin", spec.pullRef},
		{"-C", spec.destination, "checkout", "--detach", "FETCH_HEAD"},
	} {
		if err := runGit(runCtx, args, &log); err != nil {
			return req.Input, failure(spec, log.String(), err)
		}
	}
	out, err := commandRunner.Run(runCtx, []string{"-C", spec.destination, "rev-parse", "HEAD"}, "")
	appendGitLog(&log, []string{"-C", spec.destination, "rev-parse", "HEAD"}, out)
	if err != nil {
		return req.Input, failure(spec, log.String(), err)
	}
	head := strings.TrimSpace(out.Stdout)
	if spec.headSHA != "" && head != spec.headSHA {
		return req.Input, steps.StepResult{
			OK:   false,
			Data: map[string]any{"path": spec.destination, "headSHA": head},
			Artifacts: []steps.ArtifactRef{
				{Path: "git.log", Type: "git_checkout_log", Content: log.String()},
			},
			Error: &steps.StepError{Message: fmt.Sprintf("checked out HEAD %q did not match expected %q", head, spec.headSHA)},
		}
	}
	return req.Input, steps.StepResult{
		OK:   true,
		Data: map[string]any{"path": spec.destination, "headSHA": head},
		Artifacts: []steps.ArtifactRef{
			{Path: "git.log", Type: "git_checkout_log", Content: log.String()},
		},
	}
}

type spec struct {
	repoURL     string
	pullRef     string
	headSHA     string
	destination string
}

func checkoutSpec(req steps.Request) (spec, error) {
	repoURL := stringInput(req.Input, "repoURL")
	if repoURL == "" {
		return spec{}, fmt.Errorf("input.repoURL is required")
	}
	pullRef := stringInput(req.Input, "pullRef")
	if pullRef == "" {
		n, err := intInput(req.Input, "pullNumber")
		if err != nil {
			return spec{}, err
		}
		pullRef = fmt.Sprintf("refs/pull/%d/head", n)
	}
	workspace := workspacePath(req.StepCtx)
	if workspace == "" {
		return spec{}, fmt.Errorf("runloop.workspace is required")
	}
	destination, err := resolveDestination(workspace, stringInput(req.Input, "destination"))
	if err != nil {
		return spec{}, err
	}
	return spec{repoURL: repoURL, pullRef: pullRef, headSHA: stringInput(req.Input, "headSHA"), destination: destination}, nil
}

func runGit(ctx context.Context, args []string, log *strings.Builder) error {
	out, err := commandRunner.Run(ctx, args, "")
	appendGitLog(log, args, out)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("git command timed out: git %s", strings.Join(args, " "))
		}
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func appendGitLog(log *strings.Builder, args []string, out Output) {
	fmt.Fprintf(log, "$ git %s\n", strings.Join(args, " "))
	if out.Stdout != "" {
		fmt.Fprintf(log, "%s", out.Stdout)
		if !strings.HasSuffix(out.Stdout, "\n") {
			log.WriteString("\n")
		}
	}
	if out.Stderr != "" {
		fmt.Fprintf(log, "%s", out.Stderr)
		if !strings.HasSuffix(out.Stderr, "\n") {
			log.WriteString("\n")
		}
	}
}

func failure(spec spec, log string, err error) steps.StepResult {
	return steps.StepResult{
		OK:   false,
		Data: map[string]any{"path": spec.destination},
		Artifacts: []steps.ArtifactRef{
			{Path: "git.log", Type: "git_checkout_log", Content: log},
		},
		Error: &steps.StepError{Message: err.Error()},
	}
}

func stringInput(input map[string]any, key string) string {
	value, ok := input[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func intInput(input map[string]any, key string) (int, error) {
	value, ok := input[key]
	if !ok || value == nil {
		return 0, fmt.Errorf("input.%s is required", key)
	}
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("input.%s must be an integer", key)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("input.%s must be an integer", key)
	}
}

func workspacePath(ctx map[string]any) string {
	runloop, ok := ctx["runloop"].(map[string]any)
	if !ok {
		return ""
	}
	workspace, _ := runloop["workspace"].(string)
	return workspace
}

func resolveDestination(workspace, raw string) (string, error) {
	if raw == "" {
		raw = "repo"
	}
	var destination string
	if filepath.IsAbs(raw) {
		destination = filepath.Clean(raw)
	} else {
		destination = filepath.Join(workspace, raw)
	}
	rel, err := filepath.Rel(workspace, destination)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("input.destination %q escapes runloop.workspace", raw)
	}
	return destination, nil
}
