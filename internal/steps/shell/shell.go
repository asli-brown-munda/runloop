package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"runloop/internal/steps"
)

func init() { steps.Register("shell", Execute) }

func Execute(ctx context.Context, req steps.Request) (map[string]any, steps.StepResult) {
	if !req.Workflow.Permissions.Shell {
		return req.Input, steps.StepResult{OK: false, Error: &steps.StepError{Message: "shell steps are disabled unless workflow permissions.shell is true"}}
	}
	step := req.Step
	input := req.Input
	timeout := 30 * time.Second
	if step.Timeout != "" {
		parsed, err := time.ParseDuration(step.Timeout)
		if err != nil {
			return input, steps.StepResult{OK: false, Error: &steps.StepError{Message: err.Error()}}
		}
		timeout = parsed
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.Command("sh", "-c", step.Command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	env, err := steps.ResolveEnv(ctx, req, nil)
	if err != nil {
		return input, steps.StepResult{OK: false, Error: &steps.StepError{Message: err.Error()}}
	}
	cmd.Env = env
	workdir, err := steps.ResolveWorkdir(step, req.StepCtx)
	if err != nil {
		return input, steps.StepResult{OK: false, Error: &steps.StepError{Message: err.Error()}}
	}
	cmd.Dir = workdir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		data := map[string]any{
			"stdout":   "",
			"stderr":   "",
			"exitCode": -1,
		}
		return input, steps.StepResult{OK: false, Data: data, Error: &steps.StepError{Message: err.Error()}}
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	err = nil
	timedOut := false
	select {
	case err = <-done:
	case <-runCtx.Done():
		timedOut = errors.Is(runCtx.Err(), context.DeadlineExceeded)
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Process.Kill()
		}
		err = <-done
	}
	stdoutText := stdout.String()
	stderrText := stderr.String()
	data := map[string]any{
		"stdout":   stdoutText,
		"stderr":   stderrText,
		"exitCode": 0,
	}
	artifacts := []steps.ArtifactRef{
		{Path: "stdout.log", Type: "shell_stdout", Content: stdoutText},
		{Path: "stderr.log", Type: "shell_stderr", Content: stderrText},
	}
	if err != nil {
		if timedOut {
			data["exitCode"] = -1
			return input, steps.StepResult{OK: false, Data: data, Artifacts: artifacts, Error: &steps.StepError{Message: fmt.Sprintf("shell command timed out after %s", timeout)}}
		}
		if runCtx.Err() != nil {
			data["exitCode"] = -1
			return input, steps.StepResult{OK: false, Data: data, Artifacts: artifacts, Error: &steps.StepError{Message: runCtx.Err().Error()}}
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			data["exitCode"] = exitErr.ExitCode()
		}
		return input, steps.StepResult{OK: false, Data: data, Artifacts: artifacts, Error: &steps.StepError{Message: err.Error()}}
	}
	return input, steps.StepResult{OK: true, Data: data, Artifacts: artifacts}
}
