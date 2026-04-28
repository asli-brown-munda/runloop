package runs

import "time"

const (
	RunCreated   = "created"
	RunRunning   = "running"
	RunCompleted = "completed"
	RunFailed    = "failed"
	RunCancelled = "cancelled"
	RunTimedOut  = "timed_out"
	RunApproval  = "waiting_for_approval"
	RunWaitTime  = "waiting_until_time"
)

type WorkflowRun struct {
	ID                 int64
	WorkflowDispatchID int64
	WorkflowVersionID  int64
	Status             string
	StartedAt          *time.Time
	FinishedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
