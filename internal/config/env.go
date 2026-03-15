// @forge-project: forge
// @forge-path: internal/config/env.go
// Package config provides environment variable helpers for Forge.
// Follows the same pattern as Nexus and Atlas config packages.
package config

import (
	"os"
	"strings"
)

// Default values for all Forge environment variables.
const (
	DefaultHTTPAddr  = ":8082"
	DefaultNexusAddr = "http://127.0.0.1:8080"
	DefaultAtlasAddr = "http://127.0.0.1:8081"
)

// EnvOrDefault returns the value of the environment variable key,
// or defaultVal if the variable is unset or empty.
func EnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// ExpandHome replaces a leading ~ with the user's home directory.
func ExpandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return home + path[1:]
}
