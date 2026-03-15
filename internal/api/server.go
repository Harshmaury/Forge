// @forge-project: forge
// @forge-path: internal/api/server.go
// Forge HTTP API server on 127.0.0.1:8082 (ADR-003).
//
// Phase 1: POST /commands, GET /intents, GET /health
// Phase 2: POST/GET /workflows, GET /workflows/:id, POST /workflows/:id/run
// Phase 3: POST/GET /triggers, DELETE /triggers/:id
package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Harshmaury/Forge/internal/api/handler"
	forgecontext "github.com/Harshmaury/Forge/internal/context"
	"github.com/Harshmaury/Forge/internal/command"
	"github.com/Harshmaury/Forge/internal/executor"
	"github.com/Harshmaury/Forge/internal/store"
	"github.com/Harshmaury/Forge/internal/workflow"
)

// ServerConfig holds all dependencies for the Forge HTTP server.
type ServerConfig struct {
	Addr             string
	Translator       *command.Translator
	Resolver         *forgecontext.Resolver
	Engine           *executor.Engine
	Store            store.Storer
	WorkflowExecutor *workflow.Executor
	Logger           *log.Logger
}

// Server is the Forge HTTP server.
type Server struct {
	http   *http.Server
	logger *log.Logger
}

// NewServer creates the Forge HTTP server and registers all routes.
func NewServer(cfg ServerConfig) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	commandH := handler.NewCommandHandler(cfg.Translator, cfg.Resolver, cfg.Engine)
	intentsH := handler.NewIntentsHandler(cfg.Engine)

	mux := http.NewServeMux()

	// Phase 1 routes.
	mux.HandleFunc("GET /health",    handleHealth)
	mux.HandleFunc("POST /commands", commandH.Submit)
	mux.HandleFunc("GET /intents",   intentsH.List)

	// Phase 2 + 3 routes — only if store is wired.
	if cfg.Store != nil && cfg.WorkflowExecutor != nil {
		wfH      := handler.NewWorkflowHandler(cfg.Store, cfg.WorkflowExecutor, cfg.Resolver)
		triggerH := handler.NewTriggerHandler(cfg.Store)

		// Phase 2
		mux.HandleFunc("POST /workflows",          wfH.Create)
		mux.HandleFunc("GET /workflows",            wfH.List)
		mux.HandleFunc("GET /workflows/{id}",       wfH.Get)
		mux.HandleFunc("POST /workflows/{id}/run",  wfH.Run)

		// Phase 3
		mux.HandleFunc("POST /triggers",            triggerH.Create)
		mux.HandleFunc("GET /triggers",             triggerH.List)
		mux.HandleFunc("DELETE /triggers/{id}",     triggerH.Delete)
	}

	return &Server{
		http: &http.Server{
			Addr:         cfg.Addr,
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		logger: logger,
	}
}

// Run starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Printf("Forge API listening on %s", s.http.Addr)
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("forge http: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.logger.Println("Forge API shutting down...")
	return s.http.Shutdown(shutdownCtx)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true,"status":"healthy","service":"forge"}`)) //nolint:errcheck
}
