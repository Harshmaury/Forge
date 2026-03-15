// @forge-project: forge
// @forge-path: cmd/forge/main.go
// forge is the Forge execution service daemon.
// Translates developer intent into coordinated platform actions.
//
// Startup sequence:
//  1. Config (env vars)
//  2. HTTP clients (Nexus + Atlas)
//  3. Translator (command model)
//  4. Context resolver (Atlas + Nexus enrichment)
//  5. Execution engine (intent handler registry)
//  6. HTTP API server (:8082)
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Harshmaury/Forge/internal/api"
	atlasclient "github.com/Harshmaury/Forge/internal/atlas"
	"github.com/Harshmaury/Forge/internal/command"
	"github.com/Harshmaury/Forge/internal/config"
	forgecontext "github.com/Harshmaury/Forge/internal/context"
	"github.com/Harshmaury/Forge/internal/executor"
	"github.com/Harshmaury/Forge/internal/executor/intent"
	nexusclient "github.com/Harshmaury/Forge/internal/nexus"
)

const forgeVersion = "0.1.0"

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
	workspaceRoot := config.ExpandHome("~/workspace")

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// ── 2. HTTP CLIENTS ───────────────────────────────────────────────────────
	nexus := nexusclient.New(nexusAddr)
	atlas := atlasclient.New(atlasAddr)

	// Warn but continue if dependencies are unreachable at startup.
	if err := nexus.Ping(ctx); err != nil {
		logger.Printf("WARNING: Nexus not reachable at %s — context enrichment degraded: %v",
			nexusAddr, err)
	} else {
		logger.Printf("Nexus connected at %s", nexusAddr)
	}

	if err := atlas.Ping(ctx); err != nil {
		logger.Printf("WARNING: Atlas not reachable at %s — context enrichment degraded: %v",
			atlasAddr, err)
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

	// ── 6. HTTP API ───────────────────────────────────────────────────────────
	apiServer := api.NewServer(api.ServerConfig{
		Addr:       httpAddr,
		Translator: translator,
		Resolver:   resolver,
		Engine:     engine,
		Logger:     logger,
	})

	logger.Printf("✓ Forge ready — http=%s nexus=%s atlas=%s",
		httpAddr, nexusAddr, atlasAddr)

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
	logger.Println("Forge stopped cleanly")
	return nil
}
