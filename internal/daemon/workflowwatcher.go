package daemon

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"runloop/internal/workflows"
)

const defaultWorkflowReloadDebounce = 100 * time.Millisecond

type workflowLoader interface {
	LoadWorkflowFile(context.Context, string) (workflows.Version, bool, error)
}

type workflowWatcher struct {
	dir      string
	loader   workflowLoader
	logger   *slog.Logger
	debounce time.Duration
}

func newWorkflowWatcher(dir string, loader workflowLoader, logger *slog.Logger) *workflowWatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &workflowWatcher{
		dir:      dir,
		loader:   loader,
		logger:   logger,
		debounce: defaultWorkflowReloadDebounce,
	}
}

func (w *workflowWatcher) Run(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer func() {
		_ = watcher.Close()
	}()
	if err := watcher.Add(w.dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	var timer *time.Timer
	var timerC <-chan time.Time
	pending := map[string]struct{}{}
	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if !isWorkflowChangeEvent(event) || !isWorkflowYAML(event.Name) {
				continue
			}
			pending[event.Name] = struct{}{}
			if timer == nil {
				timer = time.NewTimer(w.debounce)
				timerC = timer.C
				continue
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(w.debounce)
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			w.logger.Error("workflow watcher failed", "err", err)
		case <-timerC:
			for path := range pending {
				if err := w.reloadPath(ctx, path); err != nil {
					return err
				}
				delete(pending, path)
			}
			timerC = nil
			timer = nil
		}
	}
}

func (w *workflowWatcher) reloadPath(ctx context.Context, path string) error {
	if !isWorkflowYAML(path) {
		return nil
	}
	version, created, err := w.loader.LoadWorkflowFile(ctx, path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		w.logger.Error("workflow reload failed", "path", path, "err", err)
		return nil
	}
	if created {
		w.logger.Info("workflow version reloaded", "workflow", version.Workflow.ID, "version", version.Version, "path", path)
		return nil
	}
	w.logger.Debug("workflow reload skipped unchanged content", "workflow", version.Workflow.ID, "path", path)
	return nil
}

func isWorkflowChangeEvent(event fsnotify.Event) bool {
	return event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) != 0
}

func isWorkflowYAML(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}
