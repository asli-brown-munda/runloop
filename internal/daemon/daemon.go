package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"runloop/internal/config"
	"runloop/internal/sources"
	"runloop/internal/sources/manual"
	"runloop/internal/store"
	"runloop/internal/web"
)

type Daemon struct {
	server *web.Server
	store  *store.Store
	logger *slog.Logger
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
	manager := sources.NewManager(manual.New("manual"))
	server := web.NewServer(cfg, paths, st, manager, logger)
	return &Daemon{server: server, store: st, logger: logger}, nil
}

func (d *Daemon) Run(ctx context.Context) error {
	errs := make(chan error, 1)
	go func() {
		d.logger.Info("starting local API")
		errs <- d.server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
		defer cancel()
		if err := d.server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return d.store.Close()
	case err := <-errs:
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
