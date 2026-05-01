package triggers

import (
	"context"
	"testing"

	"runloop/internal/dispatch"
	"runloop/internal/inbox"
	"runloop/internal/workflows"
)

func TestEvaluateInboxVersionRecordsEveryTriggerEvaluation(t *testing.T) {
	repo := newFakeRepo([]workflows.Version{
		workflowVersion(10, 100, "matching", workflows.Trigger{
			Type:       "inbox",
			Source:     "manual",
			EntityType: "manual_item",
			Policy:     PolicyOncePerItem,
		}),
		workflowVersion(20, 200, "non-matching", workflows.Trigger{
			Type:       "inbox",
			Source:     "filesystem",
			EntityType: "note",
			Policy:     PolicyOncePerItem,
		}),
		workflowVersion(30, 300, "manual-only", workflows.Trigger{
			Type:       "inbox",
			Source:     "manual",
			EntityType: "manual_item",
			Policy:     PolicyManualOnly,
		}),
	})
	item := inbox.InboxItem{ID: 1, SourceID: "manual", EntityType: "manual_item"}
	version := inbox.InboxItemVersion{ID: 2, InboxItemID: item.ID, Version: 1}

	if err := NewEvaluator(repo).EvaluateInboxVersion(context.Background(), item, version); err != nil {
		t.Fatal(err)
	}

	if len(repo.evaluations) != 3 {
		t.Fatalf("expected 3 evaluations, got %#v", repo.evaluations)
	}
	assertEvaluation(t, repo.evaluations[0], triggerEvaluation{
		itemID:            item.ID,
		versionID:         version.ID,
		workflowID:        10,
		workflowVersionID: 100,
		matched:           true,
		policy:            PolicyOncePerItem,
		reason:            "matched",
	})
	assertEvaluation(t, repo.evaluations[1], triggerEvaluation{
		itemID:            item.ID,
		versionID:         version.ID,
		workflowID:        20,
		workflowVersionID: 200,
		matched:           false,
		policy:            PolicyOncePerItem,
		reason:            "not matched",
	})
	assertEvaluation(t, repo.evaluations[2], triggerEvaluation{
		itemID:            item.ID,
		versionID:         version.ID,
		workflowID:        30,
		workflowVersionID: 300,
		matched:           false,
		policy:            PolicyManualOnly,
		reason:            "not matched",
	})
	if len(repo.dispatches) != 1 {
		t.Fatalf("expected one dispatch for the matching trigger, got %#v", repo.dispatches)
	}
}

func TestEvaluateInboxVersionAppliesTriggerPolicies(t *testing.T) {
	tests := []struct {
		name               string
		policy             string
		existingDispatches []dispatchKey
		firstDispatch      bool
		secondItem         inbox.InboxItem
		secondVersion      inbox.InboxItemVersion
		secondDispatch     bool
	}{
		{
			name:          "once_per_item dispatches only once for an item and workflow",
			policy:        PolicyOncePerItem,
			firstDispatch: true,
			secondItem:    inbox.InboxItem{ID: 1, SourceID: "manual", EntityType: "manual_item"},
			secondVersion: inbox.InboxItemVersion{ID: 3, InboxItemID: 1, Version: 2},
		},
		{
			name:           "once_per_version dispatches for a new inbox version",
			policy:         PolicyOncePerVersion,
			firstDispatch:  true,
			secondItem:     inbox.InboxItem{ID: 1, SourceID: "manual", EntityType: "manual_item"},
			secondVersion:  inbox.InboxItemVersion{ID: 3, InboxItemID: 1, Version: 2},
			secondDispatch: true,
		},
		{
			name:          "manual_only never dispatches",
			policy:        PolicyManualOnly,
			firstDispatch: false,
			secondItem:    inbox.InboxItem{ID: 1, SourceID: "manual", EntityType: "manual_item"},
			secondVersion: inbox.InboxItemVersion{ID: 3, InboxItemID: 1, Version: 2},
		},
		{
			name:   "empty policy defaults to once_per_item",
			policy: "",
			existingDispatches: []dispatchKey{{
				itemID:            1,
				versionID:         2,
				workflowID:        10,
				workflowVersionID: 100,
			}},
			secondItem:    inbox.InboxItem{ID: 1, SourceID: "manual", EntityType: "manual_item"},
			secondVersion: inbox.InboxItemVersion{ID: 3, InboxItemID: 1, Version: 2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo([]workflows.Version{
				workflowVersion(10, 100, "manual-workflow", workflows.Trigger{
					Type:       "inbox",
					Source:     "manual",
					EntityType: "manual_item",
					Policy:     tc.policy,
				}),
			})
			repo.seedDispatches(tc.existingDispatches...)
			item := inbox.InboxItem{ID: 1, SourceID: "manual", EntityType: "manual_item"}
			version := inbox.InboxItemVersion{ID: 2, InboxItemID: item.ID, Version: 1}
			initialDispatches := len(repo.dispatches)

			if err := NewEvaluator(repo).EvaluateInboxVersion(context.Background(), item, version); err != nil {
				t.Fatal(err)
			}
			assertDispatchDelta(t, initialDispatches, len(repo.dispatches), tc.firstDispatch)

			afterFirst := len(repo.dispatches)
			if err := NewEvaluator(repo).EvaluateInboxVersion(context.Background(), tc.secondItem, tc.secondVersion); err != nil {
				t.Fatal(err)
			}
			assertDispatchDelta(t, afterFirst, len(repo.dispatches), tc.secondDispatch)
		})
	}
}

func TestEvaluateInboxVersionDoesNotDuplicateSameVersionDispatch(t *testing.T) {
	repo := newFakeRepo([]workflows.Version{
		workflowVersion(10, 100, "manual-workflow", workflows.Trigger{
			Type:       "inbox",
			Source:     "manual",
			EntityType: "manual_item",
			Policy:     PolicyOncePerVersion,
		}),
	})
	item := inbox.InboxItem{ID: 1, SourceID: "manual", EntityType: "manual_item"}
	version := inbox.InboxItemVersion{ID: 2, InboxItemID: item.ID, Version: 1}

	if err := NewEvaluator(repo).EvaluateInboxVersion(context.Background(), item, version); err != nil {
		t.Fatal(err)
	}
	if err := NewEvaluator(repo).EvaluateInboxVersion(context.Background(), item, version); err != nil {
		t.Fatal(err)
	}

	if len(repo.dispatches) != 1 {
		t.Fatalf("expected one dispatch for repeated same-version evaluation, got %#v", repo.dispatches)
	}
	if len(repo.evaluations) != 2 {
		t.Fatalf("expected both evaluations to be recorded, got %#v", repo.evaluations)
	}
}

type triggerEvaluation struct {
	itemID            int64
	versionID         int64
	workflowID        int64
	workflowVersionID int64
	matched           bool
	policy            string
	reason            string
}

type dispatchKey struct {
	itemID            int64
	versionID         int64
	workflowID        int64
	workflowVersionID int64
}

type fakeRepo struct {
	versions    []workflows.Version
	evaluations []triggerEvaluation
	dispatches  []dispatchKey
}

func newFakeRepo(versions []workflows.Version) *fakeRepo {
	return &fakeRepo{versions: versions}
}

func (r *fakeRepo) LatestEnabledWorkflowVersions(context.Context) ([]workflows.Version, error) {
	return r.versions, nil
}

func (r *fakeRepo) RecordTriggerEvaluation(_ context.Context, itemID, versionID, workflowID, workflowVersionID int64, matched bool, policy, reason string) error {
	r.evaluations = append(r.evaluations, triggerEvaluation{
		itemID:            itemID,
		versionID:         versionID,
		workflowID:        workflowID,
		workflowVersionID: workflowVersionID,
		matched:           matched,
		policy:            policy,
		reason:            reason,
	})
	return nil
}

func (r *fakeRepo) HasDispatchForItem(_ context.Context, itemID, workflowID int64) (bool, error) {
	for _, d := range r.dispatches {
		if d.itemID == itemID && d.workflowID == workflowID {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRepo) HasDispatchForVersion(_ context.Context, versionID, workflowVersionID int64) (bool, error) {
	for _, d := range r.dispatches {
		if d.versionID == versionID && d.workflowVersionID == workflowVersionID {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRepo) CreateDispatch(_ context.Context, itemID, itemVersionID, workflowID, workflowVersionID int64) (dispatch.WorkflowDispatch, error) {
	r.dispatches = append(r.dispatches, dispatchKey{
		itemID:            itemID,
		versionID:         itemVersionID,
		workflowID:        workflowID,
		workflowVersionID: workflowVersionID,
	})
	return dispatch.WorkflowDispatch{
		ID:                 int64(len(r.dispatches)),
		InboxItemID:        itemID,
		InboxItemVersionID: itemVersionID,
		WorkflowID:         workflowID,
		WorkflowVersionID:  workflowVersionID,
		Status:             dispatch.StatusQueued,
	}, nil
}

func (r *fakeRepo) seedDispatches(dispatches ...dispatchKey) {
	r.dispatches = append(r.dispatches, dispatches...)
}

func workflowVersion(definitionID, versionID int64, workflowID string, trigger workflows.Trigger) workflows.Version {
	return workflows.Version{
		ID:           versionID,
		DefinitionID: definitionID,
		Workflow: workflows.Workflow{
			ID:       workflowID,
			Name:     workflowID,
			Triggers: []workflows.Trigger{trigger},
		},
	}
}

func assertEvaluation(t *testing.T, got, want triggerEvaluation) {
	t.Helper()
	if got != want {
		t.Fatalf("evaluation mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func assertDispatchDelta(t *testing.T, before, after int, wantDispatch bool) {
	t.Helper()
	want := before
	if wantDispatch {
		want = before + 1
	}
	if after != want {
		t.Fatalf("dispatch count before=%d after=%d, want after=%d", before, after, want)
	}
}
