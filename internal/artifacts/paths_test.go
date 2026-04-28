package artifacts

import (
	"path/filepath"
	"testing"
)

func TestArtifactPaths(t *testing.T) {
	root := t.TempDir()
	if got := InboxDir(root, 7); got != filepath.Join(root, "inbox", "inbox_7") {
		t.Fatalf("InboxDir = %q", got)
	}
	if got := StepDir(root, 3, "echo"); got != filepath.Join(root, "runs", "run_3", "steps", "echo") {
		t.Fatalf("StepDir = %q", got)
	}
}
