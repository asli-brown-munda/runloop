package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"runloop/internal/artifacts"
	"runloop/internal/config"
	"runloop/internal/inbox"
	"runloop/internal/runs"
	"runloop/internal/sources"
	_ "runloop/internal/sources/filesystem"
	"runloop/internal/sources/manual"
	_ "runloop/internal/sources/schedule"
	"runloop/internal/store"
	"runloop/internal/triggers"
	"runloop/internal/web"
)

type Daemon struct {
	server          *web.Server
	store           *store.Store
	runner          *sourceRunner
	workflowWatcher *workflowWatcher
	logger          *slog.Logger
}

func New(ctx context.Context, logger *slog.Logger) (*Daemon, error) {
	paths, err := config.DefaultPaths()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(paths.ConfigFile, paths)
	if err != nil {
		return nil, err
	}
	if err := config.EnsureDirs(paths); err != nil {
		return nil, err
	}
	st, err := store.Open(ctx, paths.DatabaseFile)
	if err != nil {
		return nil, err
	}
	if _, err := st.LoadWorkflowDir(ctx, cfg.Workflows.Dir); err != nil {
		_ = st.Close()
		return nil, err
	}
	sourcesFile, err := config.LoadSourcesFile(cfg.Sources.File)
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	manager, err := sources.LoadManager(sourcesFile)
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	if _, ok := manager.Get("manual"); !ok {
		if err := manager.Register(manual.New("manual")); err != nil {
			_ = st.Close()
			return nil, err
		}
	}
	inboxSvc := inbox.NewService(st)
	evaluator := triggers.NewEvaluator(st)
	engine := runs.NewEngine(st, artifacts.New(cfg.Daemon.ArtifactDir))
	server := web.NewServer(cfg, paths, st, manager, inboxSvc, evaluator, engine, logger)
	runner := newSourceRunner(manager, st, inboxSvc, evaluator, engine, logger)
	workflowWatcher := newWorkflowWatcher(cfg.Workflows.Dir, st, logger)
	return &Daemon{server: server, store: st, runner: runner, workflowWatcher: workflowWatcher, logger: logger}, nil
}

func (d *Daemon) Run(ctx context.Context) error {
	errs := make(chan error, 3)
	go func() {
		d.logger.Info("starting local API")
		errs <- d.server.ListenAndServe()
	}()
	runnerCtx, cancelRunner := context.WithCancel(ctx)
	defer cancelRunner()
	go func() {
		errs <- d.runner.Run(runnerCtx)
	}()
	go func() {
		errs <- d.workflowWatcher.Run(runnerCtx)
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
		defer cancel()
		if err := d.server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		cancelRunner()
		return d.store.Close()
	case err := <-errs:
		cancelRunner()
		_ = d.store.Close()
		if err != nil {
			return fmt.Errorf("daemon stopped: %w", err)
		}
		return nil
	}
}

func NewDefaultLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))
}
