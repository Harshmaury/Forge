// @forge-project: forge
// @forge-path: cmd/forge/main.go
// forge is the Forge execution service daemon.
//
// Startup sequence:
//  1. Config (env vars)
//  2. HTTP clients (Nexus + Atlas)
//  3. Translator
//  4. Context resolver
//  5. Execution engine + intent handlers
//  6. Workflow store (SQLite — Phase 2)
//  7. Workflow executor (Phase 2)
//  8. HTTP API server (:8082)
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Harshmaury/Forge/internal/api"
	atlasclient "github.com/Harshmaury/Forge/internal/atlas"
	"github.com/Harshmaury/Forge/internal/command"
	"github.com/Harshmaury/Forge/internal/config"
	forgecontext "github.com/Harshmaury/Forge/internal/context"
	"github.com/Harshmaury/Forge/internal/executor"
	"github.com/Harshmaury/Forge/internal/executor/intent"
	nexusclient "github.com/Harshmaury/Forge/internal/nexus"
	"github.com/Harshmaury/Forge/internal/store"
	"github.com/Harshmaury/Forge/internal/workflow"
)

const forgeVersion = "0.2.0"

func main() {
	logger := log.New(os.Stdout, "[forge] ", log.LstdFlags)
	logger.Printf("Forge v%s starting", forgeVersion)
	if err := run(logger); err != nil {
		logger.Fatalf("fatal: %v", err)
	}
	logger.Println("Forge stopped cleanly")
}

func run(logger *log.Logger) error {
	// ── 1. CONFIG ────────────────────────────────────────────────────────────
	httpAddr      := config.EnvOrDefault("FORGE_HTTP_ADDR", config.DefaultHTTPAddr)
	nexusAddr     := config.EnvOrDefault("NEXUS_HTTP_ADDR", config.DefaultNexusAddr)
	atlasAddr     := config.EnvOrDefault("ATLAS_HTTP_ADDR", config.DefaultAtlasAddr)
	dbPath        := config.ExpandHome(config.EnvOrDefault("FORGE_DB_PATH", "~/.nexus/forge.db"))
	workspaceRoot := config.ExpandHome("~/workspace")

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// ── 2. HTTP CLIENTS ───────────────────────────────────────────────────────
	nexus := nexusclient.New(nexusAddr)
	atlas := atlasclient.New(atlasAddr)

	if err := nexus.Ping(ctx); err != nil {
		logger.Printf("WARNING: Nexus not reachable at %s: %v", nexusAddr, err)
	} else {
		logger.Printf("Nexus connected at %s", nexusAddr)
	}
	if err := atlas.Ping(ctx); err != nil {
		logger.Printf("WARNING: Atlas not reachable at %s: %v", atlasAddr, err)
	} else {
		logger.Printf("Atlas connected at %s", atlasAddr)
	}

	// ── 3. TRANSLATOR ─────────────────────────────────────────────────────────
	translator := command.NewTranslator(workspaceRoot)

	// ── 4. CONTEXT RESOLVER ───────────────────────────────────────────────────
	resolver := forgecontext.NewResolver(nexus, atlas, logger)

	// ── 5. EXECUTION ENGINE ───────────────────────────────────────────────────
	engine := executor.NewEngine()
	engine.Register(intent.NewBuildHandler())
	engine.Register(intent.NewTestHandler())
	engine.Register(intent.NewRunHandler(nexusAddr))
	engine.Register(intent.NewDeployHandler(nexusAddr))
	logger.Printf("registered intents: %v", engine.RegisteredIntents())

	// ── 6. WORKFLOW STORE (Phase 2) ───────────────────────────────────────────
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("create db dir: %w", err)
	}
	logger.Printf("opening workflow store: %s", dbPath)
	wfStore, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("open workflow store: %w", err)
	}
	defer wfStore.Close()

	// ── 7. WORKFLOW EXECUTOR (Phase 2) ────────────────────────────────────────
	wfExecutor := workflow.NewExecutor(wfStore, engine, resolver, logger)

	// ── 8. HTTP API ───────────────────────────────────────────────────────────
	apiServer := api.NewServer(api.ServerConfig{
		Addr:             httpAddr,
		Translator:       translator,
		Resolver:         resolver,
		Engine:           engine,
		Store:            wfStore,
		WorkflowExecutor: wfExecutor,
		Logger:           logger,
	})

	logger.Printf("✓ Forge ready — http=%s nexus=%s atlas=%s db=%s",
		httpAddr, nexusAddr, atlasAddr, dbPath)

	// ── START + WAIT ──────────────────────────────────────────────────────────
	errCh := make(chan error, 1)
	go func() {
		if err := apiServer.Run(ctx); err != nil && ctx.Err() == nil {
			errCh <- fmt.Errorf("api server: %w", err)
		}
	}()

	select {
	case sig := <-sigCh:
		logger.Printf("received %s — shutting down", sig)
	case err := <-errCh:
		logger.Printf("component error: %v — shutting down", err)
	}

	cancel()
	return nil
}
