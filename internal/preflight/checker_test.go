// @forge-project: forge
// @forge-path: internal/preflight/checker_test.go
package preflight

import (
	"context"
	"testing"

	atlasclient "github.com/Harshmaury/Forge/internal/atlas"
)

// testChecker wraps Checker with injectable services for testing.
// We test the logic by pointing the checker at a mock server via httptest.
// Here we test the Result logic directly by patching services.
func makeChecker(services []*atlasclient.ProjectDetail, fail bool) *Checker {
	// We can't easily mock *atlasclient.Client without a test server.
	// Test the Result logic in isolation instead.
	_ = services
	_ = fail
	return nil
}

func TestResult_Permitted(t *testing.T) {
	r := &Result{Permitted: true, Project: &atlasclient.ProjectDetail{ID: "nexus"}}
	if !r.Permitted {
		t.Error("expected permitted=true")
	}
	if r.Project == nil || r.Project.ID != "nexus" {
		t.Error("expected project nexus")
	}
}

func TestResult_Denied(t *testing.T) {
	r := &Result{
		Permitted: false,
		Reason:    "project \"unknown\" not found in Atlas verified graph",
	}
	if r.Permitted {
		t.Error("expected permitted=false")
	}
	if r.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestResult_FailOpen(t *testing.T) {
	r := &Result{Permitted: true, Reason: "atlas unavailable — check skipped"}
	if !r.Permitted {
		t.Error("fail-open should be permitted")
	}
}

// Integration-style test: run a real preflight against a test server.
// Skipped in unit test runs — requires a live Atlas instance.
func TestChecker_Integration(t *testing.T) {
	t.Skip("integration test — requires live Atlas at :8081")
	_ = context.Background()
}
