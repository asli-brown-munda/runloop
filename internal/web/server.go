package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"runloop/internal/config"
	"runloop/internal/inbox"
	"runloop/internal/runs"
	"runloop/internal/sources"
	"runloop/internal/store"
	"runloop/internal/triggers"
)

type Server struct {
	httpServer *http.Server
}

func NewServer(cfg config.Config, paths config.Paths, st *store.Store, sourceManager *sources.Manager, inboxSvc *inbox.Service, evaluator *triggers.Evaluator, engine *runs.Engine, logger *slog.Logger) *Server {
	_ = logger
	api := &API{store: st, inbox: inboxSvc, evaluator: evaluator, engine: engine, sources: sourceManager}
	r := chi.NewRouter()
	token := readToken(paths.AuthToken)
	r.Use(authMiddleware(token))
	api.Routes(r)
	addr := fmt.Sprintf("%s:%d", cfg.Daemon.BindAddress, cfg.Daemon.Port)
	return &Server{httpServer: &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: 5 * time.Second}}
}

func (s *Server) ListenAndServe() error {
	err := s.httpServer.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func readToken(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func authMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/health" || token == "" {
				next.ServeHTTP(w, r)
				return
			}
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if got != token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
