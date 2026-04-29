package sources

import (
	"context"
	"time"
)

type Cursor struct {
	Value string
}

func (c Cursor) IsZero() bool { return c.Value == "" }

type InboxCandidate struct {
	SourceID   string
	ExternalID string
	EntityType string
	Title      string
	RawPayload map[string]any
	Normalized map[string]any
	ObservedAt time.Time
}

type Source interface {
	ID() string
	Type() string
	Sync(ctx context.Context, cursor Cursor) ([]InboxCandidate, Cursor, error)
	Test(ctx context.Context) error
}

type ChangeNotifier interface {
	WaitForChange(ctx context.Context) error
}
