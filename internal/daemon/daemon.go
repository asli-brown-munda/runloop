package daemon

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"runloop/internal/artifacts"
	"runloop/internal/config"
	"runloop/internal/inbox"
	"runloop/internal/runs"
	"runloop/internal/secrets"
	"runloop/internal/sinks"
	"runloop/internal/sources"
	_ "runloop/internal/sources/filesystem"
	"runloop/internal/sources/manual"
	_ "runloop/internal/sources/schedule"
	"runloop/internal/steps"
	_ "runloop/internal/steps/claude"
	_ "runloop/internal/steps/shell"
	_ "runloop/internal/steps/transform"
	_ "runloop/internal/steps/wait"
	"runloop/internal/store"
	"runloop/internal/triggers"
	"runloop/internal/web"
	"runloop/internal/workflows"
)

func init() {
	workflows.StepTypeValidator = steps.IsRegistered
	workflows.SinkTypeValidator = sinks.IsRegistered
}

type Daemon struct {
	server          *web.Server
	store           *store.Store
	runner          *sourceRunner
	workflowWatcher *workflowWatcher
	logger          *slog.Logger
	logFile         *os.File
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
	runtimePaths := config.ResolveRuntimePaths(paths, cfg)
	if err := config.EnsureDirs(runtimePaths); err != nil {
		return nil, err
	}
	runtimeLogger, logFile, err := newRuntimeLogger(runtimePaths.LogDir)
	if err != nil {
		return nil, err
	}
	st, err := store.Open(ctx, runtimePaths.DatabaseFile)
	if err != nil {
		_ = logFile.Close()
		return nil, err
	}
	if _, err := st.LoadWorkflowDir(ctx, cfg.Workflows.Dir); err != nil {
		_ = st.Close()
		_ = logFile.Close()
		return nil, err
	}
	sourcesFile, err := config.LoadSourcesFile(cfg.Sources.File)
	if err != nil {
		_ = st.Close()
		_ = logFile.Close()
		return nil, err
	}
	manager, err := sources.LoadManager(sourcesFile)
	if err != nil {
		_ = st.Close()
		_ = logFile.Close()
		return nil, err
	}
	if _, ok := manager.Get("manual"); !ok {
		if err := manager.Register(manual.New("manual")); err != nil {
			_ = st.Close()
			_ = logFile.Close()
			return nil, err
		}
	}
	inboxSvc := inbox.NewService(st)
	evaluator := triggers.NewEvaluator(st)
	secretResolver, err := secrets.NewFileResolver(runtimePaths.ConfigDir)
	if err != nil {
		_ = st.Close()
		_ = logFile.Close()
		return nil, err
	}
	engine := runs.NewEngine(st, artifacts.New(runtimePaths.ArtifactDir), runs.WithSecrets(secretResolver))
	server := web.NewServer(cfg, runtimePaths, st, manager, inboxSvc, evaluator, engine, secretResolver, runtimeLogger)
	runner := newSourceRunner(manager, st, inboxSvc, evaluator, engine, runtimeLogger)
	workflowWatcher := newWorkflowWatcher(cfg.Workflows.Dir, st, runtimeLogger)
	return &Daemon{server: server, store: st, runner: runner, workflowWatcher: workflowWatcher, logger: runtimeLogger, logFile: logFile}, nil
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
		return d.close()
	case err := <-errs:
		cancelRunner()
		_ = d.close()
		if err != nil {
			return fmt.Errorf("daemon stopped: %w", err)
		}
		return nil
	}
}

func NewDefaultLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))
}

func newRuntimeLogger(logDir string) (*slog.Logger, *os.File, error) {
	path := filepath.Join(logDir, "runloopd.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, err
	}
	writer := io.MultiWriter(os.Stderr, file)
	return slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{})), file, nil
}

func (d *Daemon) close() error {
	storeErr := d.store.Close()
	if d.logFile == nil {
		return storeErr
	}
	logErr := d.logFile.Close()
	if storeErr != nil {
		return storeErr
	}
	return logErr
}
