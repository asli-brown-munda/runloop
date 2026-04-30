package runs

import (
	"path/filepath"
	"testing"

	"runloop/internal/steps"
)

func TestResolveStepArtifactPathRejectsUnsafePaths(t *testing.T) {
	stepDir := filepath.Join(t.TempDir(), "runs", "run_1", "steps", "cmd")

	path, err := resolveStepArtifactPath(stepDir, steps.ArtifactRef{Path: "stdout.log"})
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(stepDir, "stdout.log") {
		t.Fatalf("path = %q", path)
	}

	for _, unsafePath := range []string{"/tmp/stdout.log", "../stdout.log", "nested/../../stdout.log"} {
		if _, err := resolveStepArtifactPath(stepDir, steps.ArtifactRef{Path: unsafePath}); err == nil {
			t.Fatalf("expected %q to be rejected", unsafePath)
		}
	}
}
