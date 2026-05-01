package runs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"runloop/internal/artifacts"
	"runloop/internal/dispatch"
	"runloop/internal/inbox"
	"runloop/internal/secrets"
	"runloop/internal/steps"
	"runloop/internal/workflows"
)

type Repository interface {
	ClaimQueuedDispatch(context.Context) (dispatch.WorkflowDispatch, bool, error)
	UpdateDispatchStatus(context.Context, int64, string) error
	CreateRun(ctx context.Context, dispatchID, workflowVersionID int64) (WorkflowRun, error)
	UpdateRunStatus(ctx context.Context, id int64, status string, errMsg string) error
	GetInboxItem(ctx context.Context, id int64) (inbox.InboxItem, error)
	LatestInboxVersion(ctx context.Context, itemID int64) (inbox.InboxItemVersion, error)
	GetWorkflowVersion(ctx context.Context, id int64) (workflows.Version, error)
	CreateStepRun(ctx context.Context, runID int64, stepID, stepType, status string, input, output map[string]any, errMsg string) (int64, error)
	AddArtifact(ctx context.Context, inboxItemID, runID, stepRunID int64, typ, path string) (int64, error)
	AddSinkOutput(ctx context.Context, runID int64, typ, path string) error
}

type Engine struct {
	repo      Repository
	artifacts *artifacts.Store
	executor  *steps.Executor
	readiness steps.ReadinessOptions
}

type EngineOption func(*engineOptions)

type engineOptions struct {
	secrets secrets.Resolver
}

func WithSecrets(resolver secrets.Resolver) EngineOption {
	return func(opts *engineOptions) { opts.secrets = resolver }
}

func NewEngine(repo Repository, artifactStore *artifacts.Store, opts ...EngineOption) *Engine {
	options := engineOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	return &Engine{
		repo:      repo,
		artifacts: artifactStore,
		executor:  steps.NewExecutor(steps.WithSecrets(options.secrets)),
		readiness: steps.ReadinessOptions{Secrets: options.secrets},
	}
}

func (e *Engine) ProcessOne(ctx context.Context) (bool, error) {
	d, ok, err := e.repo.ClaimQueuedDispatch(ctx)
	if err != nil || !ok {
		return ok, err
	}
	run, err := e.repo.CreateRun(ctx, d.ID, d.WorkflowVersionID)
	if err != nil {
		_ = e.repo.UpdateDispatchStatus(ctx, d.ID, dispatch.StatusFailed)
		return true, err
	}
	if err := e.repo.UpdateRunStatus(ctx, run.ID, RunRunning, ""); err != nil {
		return true, err
	}
	if err := e.executeRun(ctx, d, run); err != nil {
		_ = e.repo.UpdateRunStatus(ctx, run.ID, RunFailed, err.Error())
		_ = e.repo.UpdateDispatchStatus(ctx, d.ID, dispatch.StatusFailed)
		return true, err
	}
	if err := e.repo.UpdateRunStatus(ctx, run.ID, RunCompleted, ""); err != nil {
		return true, err
	}
	return true, e.repo.UpdateDispatchStatus(ctx, d.ID, dispatch.StatusCompleted)
}

func (e *Engine) Drain(ctx context.Context) error {
	for {
		processed, err := e.ProcessOne(ctx)
		if err != nil {
			return err
		}
		if !processed {
			return nil
		}
	}
}

func (e *Engine) executeRun(ctx context.Context, d dispatch.WorkflowDispatch, run WorkflowRun) error {
	item, err := e.repo.GetInboxItem(ctx, d.InboxItemID)
	if err != nil {
		return err
	}
	version, err := e.repo.LatestInboxVersion(ctx, item.ID)
	if err != nil {
		return err
	}
	wfVersion, err := e.repo.GetWorkflowVersion(ctx, d.WorkflowVersionID)
	if err != nil {
		return err
	}
	for _, diagnostic := range steps.CheckReadiness(ctx, wfVersion.Workflow, e.readiness) {
		if diagnostic.Level == steps.DiagnosticError {
			return fmt.Errorf("workflow readiness failed for step %s: %s", diagnostic.StepID, diagnostic.Message)
		}
	}
	baseCtx := map[string]any{
		"inbox": map[string]any{
			"id":         item.ID,
			"source":     item.SourceID,
			"externalID": item.ExternalID,
			"entityType": item.EntityType,
			"title":      item.Title,
			"raw":        version.RawPayload,
			"normalized": version.Normalized,
		},
	}
	root := e.artifacts.Root()
	workspace := artifacts.WorkspaceDir(root, run.ID)
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return err
	}
	baseCtx["runloop"] = map[string]any{"workspace": workspace}
	if err := e.artifacts.WriteJSON(filepath.Join(artifacts.InboxDir(root, item.ID), "raw.json"), version.RawPayload); err != nil {
		return err
	}
	if _, err := e.repo.AddArtifact(ctx, item.ID, 0, 0, "inbox_raw", filepath.Join(artifacts.InboxDir(root, item.ID), "raw.json")); err != nil {
		return err
	}
	if err := e.artifacts.WriteJSON(filepath.Join(artifacts.InboxDir(root, item.ID), "normalized.json"), version.Normalized); err != nil {
		return err
	}
	if _, err := e.repo.AddArtifact(ctx, item.ID, 0, 0, "inbox_normalized", filepath.Join(artifacts.InboxDir(root, item.ID), "normalized.json")); err != nil {
		return err
	}
	var last map[string]any
	for _, step := range wfVersion.Workflow.Steps {
		input, result := e.executor.Execute(ctx, step, wfVersion.Workflow, baseCtx)
		status := RunCompleted
		errMsg := ""
		if !result.OK {
			status = RunFailed
			if result.Error != nil {
				errMsg = result.Error.Message
			}
		}
		stepRunID, err := e.repo.CreateStepRun(ctx, run.ID, step.ID, step.Type, status, input, result.Data, errMsg)
		if err != nil {
			return err
		}
		stepDir := artifacts.StepDir(root, run.ID, step.ID)
		if err := e.artifacts.WriteJSON(filepath.Join(stepDir, "input.json"), input); err != nil {
			return err
		}
		if _, err := e.repo.AddArtifact(ctx, 0, run.ID, stepRunID, "step_input", filepath.Join(stepDir, "input.json")); err != nil {
			return err
		}
		if err := e.persistStepArtifacts(ctx, run.ID, stepRunID, stepDir, &result); err != nil {
			return err
		}
		if err := e.artifacts.WriteJSON(filepath.Join(stepDir, "output.json"), result); err != nil {
			return err
		}
		if _, err := e.repo.AddArtifact(ctx, 0, run.ID, stepRunID, "step_output", filepath.Join(stepDir, "output.json")); err != nil {
			return err
		}
		if !result.OK {
			return fmt.Errorf("step %s failed: %s", step.ID, errMsg)
		}
		last = result.Data
		baseCtx["steps"] = map[string]any{step.ID: result.Data}
	}
	return e.renderSinks(ctx, run.ID, wfVersion.Workflow, last)
}

func (e *Engine) persistStepArtifacts(ctx context.Context, runID, stepRunID int64, stepDir string, result *steps.StepResult) error {
	for i := range result.Artifacts {
		artifact := &result.Artifacts[i]
		path, err := resolveStepArtifactPath(stepDir, *artifact)
		if err != nil {
			return err
		}
		if err := e.artifacts.WriteText(path, artifact.Content); err != nil {
			return err
		}
		typ := artifact.Type
		if typ == "" {
			typ = "step_artifact"
		}
		id, err := e.repo.AddArtifact(ctx, 0, runID, stepRunID, typ, path)
		if err != nil {
			return err
		}
		artifact.ID = id
		artifact.Path = path
		artifact.Content = ""
	}
	return nil
}

func resolveStepArtifactPath(stepDir string, ref steps.ArtifactRef) (string, error) {
	if ref.Path == "" {
		return "", fmt.Errorf("step artifact path is required")
	}
	if filepath.IsAbs(ref.Path) {
		return "", fmt.Errorf("step artifact path %q must be relative", ref.Path)
	}
	clean := filepath.Clean(ref.Path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("step artifact path %q escapes step directory", ref.Path)
	}
	path := filepath.Join(stepDir, clean)
	rel, err := filepath.Rel(stepDir, path)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("step artifact path %q escapes step directory", ref.Path)
	}
	return path, nil
}

func (e *Engine) renderSinks(ctx context.Context, runID int64, wf workflows.Workflow, data map[string]any) error {
	for _, sink := range wf.Sinks {
		path := filepath.Join(artifacts.SinkDir(e.artifacts.Root(), runID), sink.Path)
		switch sink.Type {
		case "markdown":
			content := "# Runloop Report\n\n"
			for key, value := range data {
				content += fmt.Sprintf("- %s: %v\n", key, value)
			}
			if err := e.artifacts.WriteText(path, content); err != nil {
				return err
			}
		case "json", "file":
			bytes, _ := json.MarshalIndent(data, "", "  ")
			if err := e.artifacts.WriteText(path, string(bytes)+"\n"); err != nil {
				return err
			}
		}
		if err := e.repo.AddSinkOutput(ctx, runID, sink.Type, path); err != nil {
			return err
		}
		if _, err := e.repo.AddArtifact(ctx, 0, runID, 0, "sink_"+sink.Type, path); err != nil {
			return err
		}
	}
	return nil
}
