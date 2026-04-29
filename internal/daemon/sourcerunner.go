package daemon

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"runloop/internal/inbox"
	"runloop/internal/sources"
	"runloop/internal/sources/manual"
	"runloop/internal/triggers"
)

const defaultSourceInterval = 5 * time.Second

type sourceRunnerRepo interface {
	EnsureSourceRow(ctx context.Context, id, typ string) error
	GetSourceCursor(ctx context.Context, sourceID string) (sources.Cursor, error)
	UpsertSourceCursor(ctx context.Context, sourceID, cursor string) error
}

type runDrainer interface {
	Drain(ctx context.Context) error
}

type sourceRunner struct {
	manager   *sources.Manager
	repo      sourceRunnerRepo
	inbox     *inbox.Service
	evaluator *triggers.Evaluator
	engine    runDrainer
	interval  time.Duration
	logger    *slog.Logger
}

func newSourceRunner(manager *sources.Manager, repo sourceRunnerRepo, inboxSvc *inbox.Service, evaluator *triggers.Evaluator, engine runDrainer, logger *slog.Logger) *sourceRunner {
	if logger == nil {
		logger = slog.Default()
	}
	return &sourceRunner{
		manager:   manager,
		repo:      repo,
		inbox:     inboxSvc,
		evaluator: evaluator,
		engine:    engine,
		interval:  defaultSourceInterval,
		logger:    logger,
	}
}

func (r *sourceRunner) Run(ctx context.Context) error {
	if err := r.ensureSourceRows(ctx); err != nil {
		return err
	}
	for _, source := range r.manager.List() {
		if source.Type() == manual.Type {
			continue
		}
		go r.runSource(ctx, source)
	}
	<-ctx.Done()
	return nil
}

func (r *sourceRunner) ensureSourceRows(ctx context.Context) error {
	for _, source := range r.manager.List() {
		if err := r.repo.EnsureSourceRow(ctx, source.ID(), source.Type()); err != nil {
			return err
		}
	}
	return nil
}

func (r *sourceRunner) runSource(ctx context.Context, source sources.Source) {
	if err := r.syncSource(ctx, source); err != nil && !errors.Is(err, context.Canceled) {
		r.logger.Error("source sync failed", "source", source.ID(), "err", err)
	}
	if notifier, ok := source.(sources.ChangeNotifier); ok {
		r.runWatchableSource(ctx, source, notifier)
		return
	}
	r.runTimedSource(ctx, source)
}

func (r *sourceRunner) runTimedSource(ctx context.Context, source sources.Source) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.syncSource(ctx, source); err != nil && !errors.Is(err, context.Canceled) {
				r.logger.Error("source sync failed", "source", source.ID(), "err", err)
			}
		}
	}
}

func (r *sourceRunner) runWatchableSource(ctx context.Context, source sources.Source, notifier sources.ChangeNotifier) {
	for {
		err := notifier.WaitForChange(ctx)
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return
		}
		if err != nil {
			r.logger.Error("source watch failed", "source", source.ID(), "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(r.interval):
				continue
			}
		}
		if err := r.syncSource(ctx, source); err != nil && !errors.Is(err, context.Canceled) {
			r.logger.Error("source sync failed", "source", source.ID(), "err", err)
		}
	}
}

func (r *sourceRunner) syncSource(ctx context.Context, source sources.Source) error {
	cursor, err := r.repo.GetSourceCursor(ctx, source.ID())
	if err != nil {
		return err
	}
	candidates, next, err := source.Sync(ctx, cursor)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		item, version, changed, err := r.inbox.UpsertInboxItem(ctx, candidate)
		if err != nil {
			return err
		}
		if !changed {
			continue
		}
		if err := r.evaluator.EvaluateInboxVersion(ctx, item, version); err != nil {
			return err
		}
		if err := r.engine.Drain(ctx); err != nil {
			return err
		}
	}
	if next.Value != cursor.Value {
		if err := r.repo.UpsertSourceCursor(ctx, source.ID(), next.Value); err != nil {
			return err
		}
	}
	return nil
}
