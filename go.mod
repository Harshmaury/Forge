module github.com/Harshmaury/Forge

go 1.25.0

require (
	// Nexus eventbus — import topic constants only (Phase 3 automation triggers)
	github.com/Harshmaury/Nexus v0.0.0
	// UUID — unique command IDs (ADR-004 requires id field on every command)
	github.com/google/uuid v1.6.0

	// SQLite — workflow definition storage (Phase 2)
	// Same driver as Nexus and Atlas — no new toolchain dependency
	github.com/mattn/go-sqlite3 v1.14.34
)

require github.com/Harshmaury/Canon v1.0.0

// Replace directive points to local Nexus for topic constant imports
// Update to a tagged release once Nexus publishes one
replace github.com/Harshmaury/Nexus => ../nexus
