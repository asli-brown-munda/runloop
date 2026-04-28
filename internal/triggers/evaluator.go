package triggers

import (
	"context"

	"runloop/internal/dispatch"
	"runloop/internal/inbox"
	"runloop/internal/workflows"
)

type Repository interface {
	LatestEnabledWorkflowVersions(context.Context) ([]workflows.Version, error)
	RecordTriggerEvaluation(ctx context.Context, itemID, versionID, workflowID, workflowVersionID int64, matched bool, policy, reason string) error
	HasDispatchForItem(ctx context.Context, itemID, workflowID int64) (bool, error)
	HasDispatchForVersion(ctx context.Context, versionID, workflowVersionID int64) (bool, error)
	CreateDispatch(ctx context.Context, itemID, itemVersionID, workflowID, workflowVersionID int64) (dispatch.WorkflowDispatch, error)
}

type Evaluator struct {
	repo Repository
}

func NewEvaluator(repo Repository) *Evaluator {
	return &Evaluator{repo: repo}
}

func (e *Evaluator) EvaluateInboxVersion(ctx context.Context, item inbox.InboxItem, version inbox.InboxItemVersion) error {
	versions, err := e.repo.LatestEnabledWorkflowVersions(ctx)
	if err != nil {
		return err
	}
	for _, wfVersion := range versions {
		for _, trigger := range wfVersion.Workflow.Triggers {
			policy := trigger.Policy
			if policy == "" {
				policy = PolicyOncePerItem
			}
			matched := trigger.Type == "inbox" && trigger.Source == item.SourceID && trigger.EntityType == item.EntityType && policy != PolicyManualOnly
			reason := "not matched"
			if matched {
				reason = "matched"
			}
			if err := e.repo.RecordTriggerEvaluation(ctx, item.ID, version.ID, wfVersion.DefinitionID, wfVersion.ID, matched, policy, reason); err != nil {
				return err
			}
			if !matched {
				continue
			}
			allowed, err := e.policyAllowsDispatch(ctx, policy, item.ID, version.ID, wfVersion.DefinitionID, wfVersion.ID)
			if err != nil {
				return err
			}
			if !allowed {
				continue
			}
			if _, err := e.repo.CreateDispatch(ctx, item.ID, version.ID, wfVersion.DefinitionID, wfVersion.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Evaluator) policyAllowsDispatch(ctx context.Context, policy string, itemID, versionID, workflowID, workflowVersionID int64) (bool, error) {
	switch policy {
	case PolicyOncePerVersion:
		exists, err := e.repo.HasDispatchForVersion(ctx, versionID, workflowVersionID)
		return !exists, err
	case PolicyManualOnly:
		return false, nil
	default:
		exists, err := e.repo.HasDispatchForItem(ctx, itemID, workflowID)
		return !exists, err
	}
}
