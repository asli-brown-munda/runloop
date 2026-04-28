package inbox

import "time"

type InboxItem struct {
	ID         int64
	SourceID   string
	ExternalID string
	EntityType string
	Title      string
	ArchivedAt *time.Time
	IgnoredAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type InboxItemVersion struct {
	ID          int64
	InboxItemID int64
	Version     int
	RawPayload  map[string]any
	Normalized  map[string]any
	PayloadHash string
	ObservedAt  time.Time
	CreatedAt   time.Time
}
