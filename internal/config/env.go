// @forge-project: Forge
// @forge-path: internal/config/env.go
// ADR-042: GateAddr added for actor identity extraction.
package config

import (
	"os"
	"path/filepath"
)

// Default addresses — used by cmd/forge/main.go flag defaults.
const (
	DefaultHTTPAddr  = "127.0.0.1:8082"
	DefaultNexusAddr = "http://127.0.0.1:8080"
	DefaultAtlasAddr = "http://127.0.0.1:8081"
	DefaultGateAddr  = "http://127.0.0.1:8088"
)

// EnvOrDefault returns the env var value or the default if unset.
func EnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ExpandHome expands a leading ~ to the user's home directory.
func ExpandHome(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// Config holds all Forge runtime configuration.
type Config struct {
	HTTPAddr     string
	NexusAddr    string
	AtlasAddr    string
	ServiceToken string
	GateAddr     string // ADR-042: Gate service for actor identity extraction
}

// Load reads Forge configuration from environment variables.
func Load() *Config {
	return &Config{
		HTTPAddr:     EnvOrDefault("FORGE_HTTP_ADDR", DefaultHTTPAddr),
		NexusAddr:    EnvOrDefault("FORGE_NEXUS_ADDR", DefaultNexusAddr),
		AtlasAddr:    EnvOrDefault("FORGE_ATLAS_ADDR", DefaultAtlasAddr),
		ServiceToken: os.Getenv("FORGE_SERVICE_TOKEN"),
		GateAddr:     EnvOrDefault("FORGE_GATE_ADDR", DefaultGateAddr),
	}
}
