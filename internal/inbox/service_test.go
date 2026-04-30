package inbox_test

import (
	"context"
	"sync/atomic"
	"testing"

	"runloop/internal/inbox"
	"runloop/internal/sources"
)

// fakeRepo is a self-contained in-memory implementation of inbox.Repository
// that enforces the two core inbox versioning rules:
//   - dedupe by source_id + external_id
//   - new InboxItemVersion only when the payload hash changes
type fakeRepo struct {
	items    map[string]inbox.InboxItem
	versions map[int64][]inbox.InboxItemVersion
	nextID   atomic.Int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		items:    make(map[string]inbox.InboxItem),
		versions: make(map[int64][]inbox.InboxItemVersion),
	}
}

func (r *fakeRepo) UpsertInboxItem(ctx context.Context, c sources.InboxCandidate) (inbox.InboxItem, inbox.InboxItemVersion, bool, error) {
	key := c.SourceID + "\x00" + c.ExternalID
	item, exists := r.items[key]
	if !exists {
		r.nextID.Add(1)
		item = inbox.InboxItem{
			ID:         r.nextID.Load(),
			SourceID:   c.SourceID,
			ExternalID: c.ExternalID,
			EntityType: c.EntityType,
			Title:      c.Title,
		}
		r.items[key] = item
	}

	hash, err := inbox.HashPayload(c.RawPayload, c.Normalized)
	if err != nil {
		return inbox.InboxItem{}, inbox.InboxItemVersion{}, false, err
	}

	versions := r.versions[item.ID]
	if len(versions) > 0 {
		latest := versions[len(versions)-1]
		if latest.PayloadHash == hash {
			return item, latest, false, nil
		}
	}

	next := inbox.InboxItemVersion{
		ID:          int64(len(versions) + 1),
		InboxItemID: item.ID,
		Version:     len(versions) + 1,
		RawPayload:  c.RawPayload,
		Normalized:  c.Normalized,
		PayloadHash: hash,
	}
	r.versions[item.ID] = append(versions, next)
	return item, next, true, nil
}

func (r *fakeRepo) GetInboxItem(_ context.Context, id int64) (inbox.InboxItem, error) {
	for _, item := range r.items {
		if item.ID == id {
			return item, nil
		}
	}
	return inbox.InboxItem{}, nil
}

func (r *fakeRepo) ListInboxItems(_ context.Context) ([]inbox.InboxItem, error) {
	out := make([]inbox.InboxItem, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item)
	}
	return out, nil
}

func (r *fakeRepo) ArchiveInboxItem(_ context.Context, _ int64) error { return nil }
func (r *fakeRepo) IgnoreInboxItem(_ context.Context, _ int64) error  { return nil }
func (r *fakeRepo) LatestInboxVersion(_ context.Context, itemID int64) (inbox.InboxItemVersion, error) {
	versions := r.versions[itemID]
	if len(versions) == 0 {
		return inbox.InboxItemVersion{}, nil
	}
	return versions[len(versions)-1], nil
}

// TestServiceDedupesBySameSourceAndExternalID verifies that submitting the same
// source_id+external_id twice with identical payload reuses the item and version.
func TestServiceDedupesBySameSourceAndExternalID(t *testing.T) {
	ctx := context.Background()
	svc := inbox.NewService(newFakeRepo())
	payload := map[string]any{"message": "hello"}

	item1, version1, changed1, err := svc.UpsertInboxItem(ctx, sources.InboxCandidate{
		SourceID: "manual", ExternalID: "x", EntityType: "e", Title: "X",
		RawPayload: payload, Normalized: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed1 || version1.Version != 1 {
		t.Fatalf("first upsert: want changed=true version=1, got changed=%v version=%d", changed1, version1.Version)
	}

	item2, version2, changed2, err := svc.UpsertInboxItem(ctx, sources.InboxCandidate{
		SourceID: "manual", ExternalID: "x", EntityType: "e", Title: "X",
		RawPayload: payload, Normalized: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed2 {
		t.Fatal("duplicate payload: want changed=false")
	}
	if item1.ID != item2.ID {
		t.Fatalf("duplicate payload: want same item id, got %d vs %d", item1.ID, item2.ID)
	}
	if version1.ID != version2.ID {
		t.Fatalf("duplicate payload: want same version id, got %d vs %d", version1.ID, version2.ID)
	}
}

// TestServiceCreatesNewVersionOnPayloadChange verifies that a changed payload
// for the same source_id+external_id produces a new version while reusing the item.
func TestServiceCreatesNewVersionOnPayloadChange(t *testing.T) {
	ctx := context.Background()
	svc := inbox.NewService(newFakeRepo())
	base := map[string]any{"message": "hello"}

	item1, _, _, err := svc.UpsertInboxItem(ctx, sources.InboxCandidate{
		SourceID: "manual", ExternalID: "x", EntityType: "e", Title: "X",
		RawPayload: base, Normalized: base,
	})
	if err != nil {
		t.Fatal(err)
	}

	updated := map[string]any{"message": "updated"}
	item2, version2, changed, err := svc.UpsertInboxItem(ctx, sources.InboxCandidate{
		SourceID: "manual", ExternalID: "x", EntityType: "e", Title: "X",
		RawPayload: updated, Normalized: updated,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("changed payload: want changed=true")
	}
	if item1.ID != item2.ID {
		t.Fatalf("changed payload: want same item id, got %d vs %d", item1.ID, item2.ID)
	}
	if version2.Version != 2 {
		t.Fatalf("changed payload: want version=2, got %d", version2.Version)
	}
}
