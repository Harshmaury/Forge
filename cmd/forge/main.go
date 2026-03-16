// @forge-project: forge
// @forge-path: cmd/forge/main.go
// FG-H-05: workspaceRoot now read from FORGE_WORKSPACE env var.
//   Previously hardcoded to ~/workspace — inconsistent with Atlas
//   (ATLAS_WORKSPACE) and Nexus (NEXUS_WORKSPACE). Each service
//   independently configures its workspace root via env vars.
//
// forge is the Forge execution service daemon.
//
// Startup sequence:
//  1. Config
//  2. HTTP clients (Nexus + Atlas)
//  3. Translator + context resolver
//  4. Execution engine + intent handlers
//  5. Workflow store (SQLite)
//  6. Workflow executor
//  7. Trigger registry + subscriber (Phase 3)
//  8. HTTP API server (:8082)
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
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
	"github.com/Harshmaury/Forge/internal/trigger"
	"github.com/Harshmaury/Forge/internal/workflow"
)

const forgeVersion = "0.3.0"

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
	workspaceRoot := config.ExpandHome(config.EnvOrDefault("FORGE_WORKSPACE", "~/workspace"))

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

	// ── 3. TRANSLATOR + RESOLVER ──────────────────────────────────────────────
	translator := command.NewTranslator(workspaceRoot)
	resolver   := forgecontext.NewResolver(nexus, atlas, logger)

	// ── 4. EXECUTION ENGINE ───────────────────────────────────────────────────
	engine := executor.NewEngine()
	engine.Register(intent.NewBuildHandler())
	engine.Register(intent.NewTestHandler())
	engine.Register(intent.NewRunHandler(nexusAddr))
	engine.Register(intent.NewDeployHandler(nexusAddr))
	logger.Printf("registered intents: %v", engine.RegisteredIntents())

	// ── 5. WORKFLOW STORE ────────────────────────────────────────────────────
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("create db dir: %w", err)
	}
	logger.Printf("opening workflow store: %s", dbPath)
	wfStore, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("open workflow store: %w", err)
	}
	defer wfStore.Close()

	// ── 6. WORKFLOW EXECUTOR ──────────────────────────────────────────────────
	wfExecutor := workflow.NewExecutor(wfStore, engine, resolver, logger)

	// ── 7. TRIGGER REGISTRY + SUBSCRIBER (Phase 3) ───────────────────────────
	triggerRegistry   := trigger.NewRegistry(wfStore)
	triggerSubscriber := trigger.NewSubscriber(nexusAddr, triggerRegistry, wfExecutor, logger)

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

	// ── START GOROUTINES ──────────────────────────────────────────────────────
	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := apiServer.Run(ctx); err != nil && ctx.Err() == nil {
			errCh <- fmt.Errorf("api server: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := triggerSubscriber.Run(ctx); err != nil && ctx.Err() == nil {
			errCh <- fmt.Errorf("trigger subscriber: %w", err)
		}
	}()

	// ── WAIT FOR SHUTDOWN ─────────────────────────────────────────────────────
	select {
	case sig := <-sigCh:
		logger.Printf("received %s — shutting down", sig)
	case err := <-errCh:
		logger.Printf("component error: %v — shutting down", err)
	}

	cancel()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	<-done

	return nil
}
