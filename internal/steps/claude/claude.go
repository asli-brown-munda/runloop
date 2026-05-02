package claude

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"runloop/internal/secrets"
	"runloop/internal/steps"
)

func init() { steps.Register("claude", Execute) }

type Output struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Runner interface {
	Run(ctx context.Context, command string, args []string, env []string, dir string) (Output, error)
}

var commandRunner Runner = execRunner{}

const anthropicAPIKeyEnv = "ANTHROPIC_API_KEY"

type execRunner struct{}

func (execRunner) Run(ctx context.Context, command string, args []string, env []string, dir string) (Output, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = env
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
		return req.Input, steps.StepResult{OK: false, Error: &steps.StepError{Message: "claude steps are disabled unless workflow permissions.shell is true"}}
	}
	timeout := 30 * time.Second
	if req.Step.Timeout != "" {
		parsed, err := time.ParseDuration(req.Step.Timeout)
		if err != nil {
			return req.Input, steps.StepResult{OK: false, Error: &steps.StepError{Message: err.Error()}}
		}
		timeout = parsed
	}
	prompt, err := renderPrompt(req)
	if err != nil {
		return req.Input, steps.StepResult{OK: false, Error: &steps.StepError{Message: err.Error()}}
	}
	args := []string{"-p"}
	permissionMode := req.Step.PermissionMode
	if permissionMode == "" {
		permissionMode = "plan"
	}
	args = append(args, "--permission-mode", permissionMode)
	if req.Step.Model != "" {
		args = append(args, "--model", req.Step.Model)
	}
	args = append(args, req.Step.Args...)
	args = append(args, prompt)

	envExtra, err := claudeEnv(ctx, req)
	if err != nil {
		return req.Input, steps.StepResult{OK: false, Error: &steps.StepError{Message: err.Error()}}
	}
	env, err := steps.ResolveEnv(ctx, req, envExtra)
	if err != nil {
		return req.Input, steps.StepResult{OK: false, Error: &steps.StepError{Message: err.Error()}}
	}
	workdir, err := steps.ResolveWorkdir(req.Step, req.StepCtx)
	if err != nil {
		return req.Input, steps.StepResult{OK: false, Error: &steps.StepError{Message: err.Error()}}
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := commandRunner.Run(runCtx, "claude", args, env, workdir)
	data := map[string]any{"stdout": out.Stdout, "stderr": out.Stderr, "exitCode": out.ExitCode}
	artifacts := []steps.ArtifactRef{
		{Path: "stdout.log", Type: "claude_stdout", Content: out.Stdout},
		{Path: "stderr.log", Type: "claude_stderr", Content: out.Stderr},
	}
	if err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			data["exitCode"] = -1
			return req.Input, steps.StepResult{OK: false, Data: data, Artifacts: artifacts, Error: &steps.StepError{Message: fmt.Sprintf("claude command timed out after %s", timeout)}}
		}
		return req.Input, steps.StepResult{OK: false, Data: data, Artifacts: artifacts, Error: &steps.StepError{Message: err.Error()}}
	}
	return req.Input, steps.StepResult{OK: true, Data: data, Artifacts: artifacts}
}

func renderPrompt(req steps.Request) (string, error) {
	rendered, err := steps.RenderValue(req.Step.Prompt, req.StepCtx)
	if err != nil {
		return "", err
	}
	prompt, ok := rendered.(string)
	if !ok {
		return "", fmt.Errorf("prompt must render to string")
	}
	return prompt, nil
}

func claudeEnv(ctx context.Context, req steps.Request) (map[string]string, error) {
	auth := req.Step.Auth
	if auth == "" {
		auth = "auto"
	}
	switch auth {
	case "login":
		return nil, nil
	case "apiKey":
		if req.Step.Connection != "" {
			value, err := resolveConnectionAPIKey(ctx, req, req.Step.Connection)
			if err != nil {
				return nil, err
			}
			return map[string]string{anthropicAPIKeyEnv: value}, nil
		}
		if req.Secrets == nil {
			return nil, fmt.Errorf("claude API key auth requires profiles.claude env ANTHROPIC_API_KEY in secrets.yaml")
		}
		value, err := req.Secrets.ResolveProfileEnv(ctx, "claude", anthropicAPIKeyEnv)
		if err != nil {
			return nil, err
		}
		return map[string]string{anthropicAPIKeyEnv: value}, nil
	case "auto":
		if req.Step.Connection != "" {
			value, err := resolveConnectionAPIKey(ctx, req, req.Step.Connection)
			if err != nil {
				return nil, err
			}
			return map[string]string{anthropicAPIKeyEnv: value}, nil
		}
		if req.Secrets == nil {
			return nil, nil
		}
		if inspector, ok := req.Secrets.(secrets.ProfileInspector); ok && !inspector.ProfileEnvConfigured("claude", anthropicAPIKeyEnv) {
			return nil, nil
		}
		value, err := req.Secrets.ResolveProfileEnv(ctx, "claude", anthropicAPIKeyEnv)
		if err != nil {
			return nil, err
		}
		return map[string]string{anthropicAPIKeyEnv: value}, nil
	default:
		return nil, fmt.Errorf("claude auth %q is unsupported", auth)
	}
}

func resolveConnectionAPIKey(ctx context.Context, req steps.Request, connection string) (string, error) {
	if req.Secrets == nil {
		return "", fmt.Errorf("claude connection %q requires a secrets resolver that can resolve %s", connection, anthropicAPIKeyEnv)
	}
	resolver, ok := req.Secrets.(secrets.ConnectionEnvResolver)
	if !ok {
		return "", fmt.Errorf("claude connection %q requires a connection env resolver for %s", connection, anthropicAPIKeyEnv)
	}
	value, err := resolver.ResolveConnectionEnv(ctx, connection, anthropicAPIKeyEnv)
	if err != nil {
		return "", fmt.Errorf("claude connection %q cannot resolve %s: %w", connection, anthropicAPIKeyEnv, err)
	}
	return value, nil
}
