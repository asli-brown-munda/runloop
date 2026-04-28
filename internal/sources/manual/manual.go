package manual

import (
	"context"
	"time"

	"runloop/internal/sources"
)

const (
	Type       = "manual"
	EntityType = "manual_item"
)

func init() {
	sources.Register(Type, func(id string, _ map[string]any) (sources.Source, error) {
		return New(id), nil
	})
}

type Source struct {
	id string
}

func New(id string) Source {
	if id == "" {
		id = "manual"
	}
	return Source{id: id}
}

func (s Source) ID() string { return s.id }

func (s Source) Type() string { return Type }

func (s Source) Sync(ctx context.Context, cursor sources.Cursor) ([]sources.InboxCandidate, sources.Cursor, error) {
	return nil, cursor, ctx.Err()
}

func (s Source) Test(ctx context.Context) error {
	return ctx.Err()
}

func Candidate(sourceID, externalID, title string, payload map[string]any) sources.InboxCandidate {
	if payload == nil {
		payload = map[string]any{}
	}
	return sources.InboxCandidate{
		SourceID:   sourceID,
		ExternalID: externalID,
		EntityType: EntityType,
		Title:      title,
		RawPayload: payload,
		Normalized: payload,
		ObservedAt: time.Now().UTC(),
	}
}
