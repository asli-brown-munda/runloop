package daemon

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"runloop/internal/sources"
)

type fakeSourceRepo struct {
	mu      sync.Mutex
	cursors map[string]sources.Cursor
}

func newFakeSourceRepo() *fakeSourceRepo {
	return &fakeSourceRepo{cursors: map[string]sources.Cursor{}}
}

func (r *fakeSourceRepo) EnsureSourceRow(ctx context.Context, id, typ string) error {
	return ctx.Err()
}

func (r *fakeSourceRepo) GetSourceCursor(ctx context.Context, sourceID string) (sources.Cursor, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cursors[sourceID], ctx.Err()
}

func (r *fakeSourceRepo) UpsertSourceCursor(ctx context.Context, sourceID, cursor string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cursors[sourceID] = sources.Cursor{Value: cursor}
	return ctx.Err()
}

type fakeRunDrainer struct{}

func (fakeRunDrainer) Drain(ctx context.Context) error { return ctx.Err() }

type watchableTestSource struct {
	id      string
	waitc   chan struct{}
	syncs   atomic.Int32
	waiting atomic.Int32
}

func (s *watchableTestSource) ID() string   { return s.id }
func (s *watchableTestSource) Type() string { return "watchable" }

func (s *watchableTestSource) Sync(ctx context.Context, cursor sources.Cursor) ([]sources.InboxCandidate, sources.Cursor, error) {
	s.syncs.Add(1)
	return nil, cursor, ctx.Err()
}

func (s *watchableTestSource) Test(ctx context.Context) error { return ctx.Err() }

func (s *watchableTestSource) WaitForChange(ctx context.Context) error {
	s.waiting.Store(1)
	select {
	case <-s.waitc:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestRunSyncsWatchableSourceAfterChange(t *testing.T) {
	source := &watchableTestSource{id: "watched", waitc: make(chan struct{})}
	runner := newSourceRunner(sources.NewManager(source), newFakeSourceRepo(), nil, nil, fakeRunDrainer{}, nil)
	runner.interval = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errc := make(chan error, 1)
	go func() {
		errc <- runner.Run(ctx)
	}()

	waitForAtomic(t, &source.syncs, 1, "initial sync")
	waitForAtomic(t, &source.waiting, 1, "watch wait")
	assertAtomicStays(t, &source.syncs, 1, 20*time.Millisecond, "source synced without a change event")

	source.waitc <- struct{}{}
	waitForAtomic(t, &source.syncs, 2, "event sync")

	cancel()
	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}

func waitForAtomic(t *testing.T, value *atomic.Int32, want int32, reason string) {
	t.Helper()
	deadline := time.After(time.Second)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if value.Load() >= want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %s: got %d, want at least %d", reason, value.Load(), want)
		case <-tick.C:
		}
	}
}

func assertAtomicStays(t *testing.T, value *atomic.Int32, want int32, duration time.Duration, reason string) {
	t.Helper()
	timer := time.NewTimer(duration)
	defer timer.Stop()
	<-timer.C
	if got := value.Load(); got != want {
		t.Fatalf("%s: got %d, want %d", reason, got, want)
	}
}
