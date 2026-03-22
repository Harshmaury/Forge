// @forge-project: forge
// @forge-path: internal/store/dedup_test.go
// CW-5: unit tests for command idempotency dedup store methods.
package store

import (
	"os"
	"testing"
	"time"
)

// newTestStore creates an in-memory (temp file) Forge store for testing.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	f, err := os.CreateTemp("", "forge-dedup-test-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	s, err := New(f.Name())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSetAndGetDedupRecord(t *testing.T) {
	s := newTestStore(t)

	rec := &DedupRecord{
		CommandID:  "cmd-abc-123",
		ResultJSON: `{"command_id":"cmd-abc-123","success":true}`,
		ExpiresAt:  time.Now().UTC().Add(5 * time.Minute),
	}

	if err := s.SetDedupRecord(rec); err != nil {
		t.Fatalf("SetDedupRecord: %v", err)
	}

	got, err := s.GetDedupRecord("cmd-abc-123")
	if err != nil {
		t.Fatalf("GetDedupRecord: %v", err)
	}
	if got == nil {
		t.Fatal("expected record, got nil")
	}
	if got.CommandID != rec.CommandID {
		t.Errorf("CommandID: got %q, want %q", got.CommandID, rec.CommandID)
	}
	if got.ResultJSON != rec.ResultJSON {
		t.Errorf("ResultJSON: got %q, want %q", got.ResultJSON, rec.ResultJSON)
	}
}

func TestGetDedupRecord_NotFound(t *testing.T) {
	s := newTestStore(t)

	got, err := s.GetDedupRecord("nonexistent-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for missing record, got %+v", got)
	}
}

func TestGetDedupRecord_Expired(t *testing.T) {
	s := newTestStore(t)

	// Insert a record that is already expired.
	rec := &DedupRecord{
		CommandID:  "cmd-expired",
		ResultJSON: `{"command_id":"cmd-expired","success":true}`,
		ExpiresAt:  time.Now().UTC().Add(-1 * time.Second), // past
	}
	if err := s.SetDedupRecord(rec); err != nil {
		t.Fatalf("SetDedupRecord: %v", err)
	}

	got, err := s.GetDedupRecord("cmd-expired")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for expired record, got %+v", got)
	}
}

func TestSetDedupRecord_Upsert(t *testing.T) {
	s := newTestStore(t)

	first := &DedupRecord{
		CommandID:  "cmd-upsert",
		ResultJSON: `{"success":true,"output":"first"}`,
		ExpiresAt:  time.Now().UTC().Add(5 * time.Minute),
	}
	if err := s.SetDedupRecord(first); err != nil {
		t.Fatalf("SetDedupRecord first: %v", err)
	}

	// Re-insert with different result — upsert must overwrite.
	second := &DedupRecord{
		CommandID:  "cmd-upsert",
		ResultJSON: `{"success":true,"output":"second"}`,
		ExpiresAt:  time.Now().UTC().Add(5 * time.Minute),
	}
	if err := s.SetDedupRecord(second); err != nil {
		t.Fatalf("SetDedupRecord second: %v", err)
	}

	got, err := s.GetDedupRecord("cmd-upsert")
	if err != nil {
		t.Fatalf("GetDedupRecord: %v", err)
	}
	if got == nil {
		t.Fatal("expected record after upsert, got nil")
	}
	if got.ResultJSON != second.ResultJSON {
		t.Errorf("expected second result after upsert, got %q", got.ResultJSON)
	}
}
