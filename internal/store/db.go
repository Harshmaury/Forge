// @forge-project: forge
// @forge-path: internal/store/db.go
// Package store manages the Forge SQLite workflow database.
//
// Phase 2 (migration v1): workflows + workflow_steps
// Phase 3 (migration v2): triggers
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ── STORE ─────────────────────────────────────────────────────────────────────

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open forge db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping forge db: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("forge migrations: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// ── WORKFLOWS ─────────────────────────────────────────────────────────────────

func (s *Store) CreateWorkflow(w *Workflow) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`
		INSERT INTO workflows (id, name, description, trigger, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, w.ID, w.Name, w.Description, w.Trigger, now, now)
	if err != nil {
		return fmt.Errorf("create workflow %s: %w", w.ID, err)
	}
	return nil
}

func (s *Store) GetWorkflow(id string) (*Workflow, error) {
	row := s.db.QueryRow(`
		SELECT id, name, description, trigger, created_at, updated_at
		FROM workflows WHERE id = ?
	`, id)
	w := &Workflow{}
	err := row.Scan(&w.ID, &w.Name, &w.Description, &w.Trigger, &w.CreatedAt, &w.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get workflow %s: %w", id, err)
	}
	return w, nil
}

func (s *Store) GetAllWorkflows() ([]*Workflow, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, trigger, created_at, updated_at
		FROM workflows ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("get all workflows: %w", err)
	}
	defer rows.Close()
	var workflows []*Workflow
	for rows.Next() {
		w := &Workflow{}
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.Trigger,
			&w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workflow: %w", err)
		}
		workflows = append(workflows, w)
	}
	return workflows, rows.Err()
}

func (s *Store) DeleteWorkflow(id string) error {
	if err := s.DeleteSteps(id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM workflows WHERE id = ?`, id)
	return err
}

// ── STEPS ─────────────────────────────────────────────────────────────────────

func (s *Store) AddStep(step *WorkflowStep) error {
	params, err := json.Marshal(step.Parameters)
	if err != nil {
		return fmt.Errorf("marshal step parameters: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT INTO workflow_steps (workflow_id, position, intent, target, parameters)
		VALUES (?, ?, ?, ?, ?)
	`, step.WorkflowID, step.Position, step.Intent, step.Target, string(params))
	if err != nil {
		return fmt.Errorf("add step to workflow %s pos %d: %w",
			step.WorkflowID, step.Position, err)
	}
	return nil
}

func (s *Store) GetSteps(workflowID string) ([]*WorkflowStep, error) {
	rows, err := s.db.Query(`
		SELECT id, workflow_id, position, intent, target, parameters
		FROM workflow_steps WHERE workflow_id = ? ORDER BY position ASC
	`, workflowID)
	if err != nil {
		return nil, fmt.Errorf("get steps for workflow %s: %w", workflowID, err)
	}
	defer rows.Close()
	var steps []*WorkflowStep
	for rows.Next() {
		step := &WorkflowStep{}
		var paramsJSON string
		if err := rows.Scan(&step.ID, &step.WorkflowID, &step.Position,
			&step.Intent, &step.Target, &paramsJSON); err != nil {
			return nil, fmt.Errorf("scan step: %w", err)
		}
		if err := json.Unmarshal([]byte(paramsJSON), &step.Parameters); err != nil {
			step.Parameters = map[string]string{}
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (s *Store) DeleteSteps(workflowID string) error {
	_, err := s.db.Exec(`DELETE FROM workflow_steps WHERE workflow_id = ?`, workflowID)
	return err
}

// ── TRIGGERS (PHASE 3) ────────────────────────────────────────────────────────

func (s *Store) CreateTrigger(t *Trigger) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`
		INSERT INTO triggers (id, event, workflow_id, filter_ext, filter_proj, filter_dir, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.Event, t.WorkflowID, t.FilterExt, t.FilterProj, t.FilterDir, t.Enabled, now)
	if err != nil {
		return fmt.Errorf("create trigger %s: %w", t.ID, err)
	}
	return nil
}

func (s *Store) GetTrigger(id string) (*Trigger, error) {
	row := s.db.QueryRow(`
		SELECT id, event, workflow_id, filter_ext, filter_proj, filter_dir, enabled, created_at
		FROM triggers WHERE id = ?
	`, id)
	t := &Trigger{}
	err := row.Scan(&t.ID, &t.Event, &t.WorkflowID, &t.FilterExt,
		&t.FilterProj, &t.FilterDir, &t.Enabled, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get trigger %s: %w", id, err)
	}
	return t, nil
}

func (s *Store) GetAllTriggers() ([]*Trigger, error) {
	rows, err := s.db.Query(`
		SELECT id, event, workflow_id, filter_ext, filter_proj, filter_dir, enabled, created_at
		FROM triggers ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("get all triggers: %w", err)
	}
	defer rows.Close()
	return scanTriggers(rows)
}

func (s *Store) GetEnabledTriggersByEvent(event string) ([]*Trigger, error) {
	rows, err := s.db.Query(`
		SELECT id, event, workflow_id, filter_ext, filter_proj, filter_dir, enabled, created_at
		FROM triggers WHERE event = ? AND enabled = 1
	`, event)
	if err != nil {
		return nil, fmt.Errorf("get triggers for event %s: %w", event, err)
	}
	defer rows.Close()
	return scanTriggers(rows)
}

func (s *Store) DeleteTrigger(id string) error {
	_, err := s.db.Exec(`DELETE FROM triggers WHERE id = ?`, id)
	return err
}

// ── MIGRATIONS ────────────────────────────────────────────────────────────────

type schemaVersion struct {
	version int
	up      string
}

var allMigrations = []schemaVersion{
	// v1 — workflow storage (Phase 2)
	{1, `CREATE TABLE IF NOT EXISTS workflows (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		trigger     TEXT NOT NULL DEFAULT 'manual',
		created_at  DATETIME NOT NULL,
		updated_at  DATETIME NOT NULL
	)`},
	{1, `CREATE TABLE IF NOT EXISTS workflow_steps (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		workflow_id TEXT    NOT NULL,
		position    INTEGER NOT NULL,
		intent      TEXT    NOT NULL,
		target      TEXT    NOT NULL,
		parameters  TEXT    NOT NULL DEFAULT '{}',
		FOREIGN KEY (workflow_id) REFERENCES workflows(id)
	)`},
	{1, `CREATE INDEX IF NOT EXISTS idx_steps_workflow ON workflow_steps(workflow_id)`},
	{1, `CREATE UNIQUE INDEX IF NOT EXISTS idx_steps_position ON workflow_steps(workflow_id, position)`},

	// v2 — automation triggers (Phase 3)
	{2, `CREATE TABLE IF NOT EXISTS triggers (
		id          TEXT    PRIMARY KEY,
		event       TEXT    NOT NULL DEFAULT '',
		workflow_id TEXT    NOT NULL DEFAULT '',
		filter_ext  TEXT    NOT NULL DEFAULT '',
		filter_proj TEXT    NOT NULL DEFAULT '',
		filter_dir  TEXT    NOT NULL DEFAULT '',
		enabled     INTEGER NOT NULL DEFAULT 1,
		created_at  DATETIME NOT NULL,
		FOREIGN KEY (workflow_id) REFERENCES workflows(id)
	)`},
	{2, `CREATE INDEX IF NOT EXISTS idx_triggers_event   ON triggers(event)`},
	{2, `CREATE INDEX IF NOT EXISTS idx_triggers_enabled ON triggers(enabled)`},
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			applied_at DATETIME NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var current int
	if err := s.db.QueryRow(
		`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	applied := map[int]bool{}
	for _, m := range allMigrations {
		if m.version <= current {
			continue
		}
		if _, err := s.db.Exec(m.up); err != nil {
			return fmt.Errorf("migration v%d: %w\nSQL: %s", m.version, err, m.up)
		}
		if !applied[m.version] {
			applied[m.version] = true
			if _, err := s.db.Exec(
				`INSERT OR IGNORE INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
				m.version, time.Now().UTC(),
			); err != nil {
				return fmt.Errorf("record migration v%d: %w", m.version, err)
			}
		}
	}
	return nil
}

// ── SCAN HELPERS ─────────────────────────────────────────────────────────────

func scanTriggers(rows *sql.Rows) ([]*Trigger, error) {
	var triggers []*Trigger
	for rows.Next() {
		t := &Trigger{}
		if err := rows.Scan(&t.ID, &t.Event, &t.WorkflowID, &t.FilterExt,
			&t.FilterProj, &t.FilterDir, &t.Enabled, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan trigger: %w", err)
		}
		triggers = append(triggers, t)
	}
	return triggers, rows.Err()
}
