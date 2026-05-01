package artifacts

import (
	"fmt"
	"path/filepath"
)

func InboxDir(root string, inboxID int64) string {
	return filepath.Join(root, "inbox", fmt.Sprintf("inbox_%d", inboxID))
}

func RunDir(root string, runID int64) string {
	return filepath.Join(root, "runs", fmt.Sprintf("run_%d", runID))
}

func StepDir(root string, runID int64, stepID string) string {
	return filepath.Join(RunDir(root, runID), "steps", stepID)
}

func WorkspaceDir(root string, runID int64) string {
	return filepath.Join(RunDir(root, runID), "workspace")
}

func SinkDir(root string, runID int64) string {
	return filepath.Join(RunDir(root, runID), "sinks")
}
