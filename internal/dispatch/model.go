package dispatch

import "time"

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

type WorkflowDispatch struct {
	ID                 int64
	InboxItemID        int64
	InboxItemVersionID int64
	WorkflowID         int64
	WorkflowVersionID  int64
	Status             string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
