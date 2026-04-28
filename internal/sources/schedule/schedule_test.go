package schedule

import (
	"context"
	"testing"
	"time"

	"runloop/internal/sources"
)

func TestScheduleEveryEmitsAfterInterval(t *testing.T) {
	src, err := New("heartbeat", map[string]any{"every": "10ms"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	base := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	src.now = func() time.Time { return base }

	candidates, cursor, err := src.Sync(context.Background(), sources.Cursor{})
	if err != nil {
		t.Fatalf("baseline Sync: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("baseline Sync should not emit, got %d", len(candidates))
	}
	if cursor.IsZero() {
		t.Fatal("baseline Sync should set cursor")
	}

	src.now = func() time.Time { return base.Add(50 * time.Millisecond) }
	candidates, cursor2, err := src.Sync(context.Background(), cursor)
	if err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate after interval")
	}
	if candidates[0].EntityType != defaultEntityType {
		t.Fatalf("unexpected entityType %q", candidates[0].EntityType)
	}
	if cursor2.Value == cursor.Value {
		t.Fatal("cursor should advance after firing")
	}
}

func TestScheduleCatchUpIsCapped(t *testing.T) {
	src, err := New("heartbeat", map[string]any{"every": "1ms"})
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	src.now = func() time.Time { return base.Add(time.Second) }

	candidates, _, err := src.Sync(context.Background(), sources.Cursor{Value: base.Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != maxCatchUp {
		t.Fatalf("expected catch-up to cap at %d, got %d", maxCatchUp, len(candidates))
	}
}

func TestScheduleCronEmitsForPastCursor(t *testing.T) {
	src, err := New("heartbeat", map[string]any{"cron": "* * * * *"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	base := time.Date(2026, 4, 28, 12, 0, 30, 0, time.UTC)
	src.now = func() time.Time { return base }

	cursor := sources.Cursor{Value: base.Add(-90 * time.Second).Format(time.RFC3339Nano)}
	candidates, _, err := src.Sync(context.Background(), cursor)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected at least one cron tick for 90s gap")
	}
}

func TestScheduleRejectsBothEveryAndCron(t *testing.T) {
	if _, err := New("heartbeat", map[string]any{"every": "1m", "cron": "* * * * *"}); err == nil {
		t.Fatal("expected error when both every and cron are set")
	}
}

func TestScheduleRequiresOne(t *testing.T) {
	if _, err := New("heartbeat", map[string]any{}); err == nil {
		t.Fatal("expected error when neither every nor cron is set")
	}
}
