module github.com/Harshmaury/Forge

go 1.23.0

require (
	// UUID — unique command IDs (ADR-004 requires id field on every command)
	github.com/google/uuid v1.6.0

	// SQLite — workflow definition storage (Phase 2)
	// Same driver as Nexus and Atlas — no new toolchain dependency
	github.com/mattn/go-sqlite3 v1.14.34

	// YAML — workflow definition files (Phase 2)
	gopkg.in/yaml.v3 v3.0.1

	// CLI — forge subcommands (same library as engx in Nexus)
	github.com/spf13/cobra v1.10.2

	// Nexus eventbus — import topic constants only (Phase 3 automation triggers)
	github.com/Harshmaury/Nexus v0.0.0
)

// Replace directive points to local Nexus for topic constant imports
// Update to a tagged release once Nexus publishes one
replace github.com/Harshmaury/Nexus => ../nexus
