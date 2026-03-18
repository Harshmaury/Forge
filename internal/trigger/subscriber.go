// @forge-project: forge
// @forge-path: internal/trigger/subscriber.go
// FG-Fix-02: dispatch now uses a bounded semaphore to cap concurrent
//   workflow goroutines. Previously each matched trigger spawned an
//   unbounded goroutine — a git checkout touching many files could fire
//   50+ concurrent build/test processes (fork bomb under load).
//
//   A buffered channel of size maxConcurrentWorkflows acts as a semaphore:
//   acquiring a slot (send) before launching, releasing (recv) on completion.
//   If all slots are taken the trigger is dropped and logged — workflows
//   are best-effort automation, not guaranteed delivery.
//
// Subscriber polls the Nexus events API for workspace events and
// fires matching workflows via the workflow executor.
//
// Polling pattern mirrors Atlas internal/nexus/subscriber.go (ADR-007):
//   GET /events?limit=50&since=<last_id> every 3 seconds.
//
// Import rule: topic constants imported from pkg/events only (ADR-007).
package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	canon "github.com/Harshmaury/Canon/identity"
	"strconv"
	"time"

	"github.com/Harshmaury/Forge/internal/command"
	"github.com/Harshmaury/Forge/internal/workflow"
	nexusevents "github.com/Harshmaury/Nexus/pkg/events"
)

// ── CONSTANTS ─────────────────────────────────────────────────────────────────

const (
	pollInterval            = 3 * time.Second
	pollEventLimit          = 50

	// maxConcurrentWorkflows caps the number of workflow goroutines running
	// simultaneously. Prevents a burst of workspace events from spawning
	// an unbounded number of build/test processes.
	maxConcurrentWorkflows = 8
)

// ── NEXUS CLIENT INTERFACE ────────────────────────────────────────────────────

// EventPoller is the subset of the Nexus HTTP client used by the Subscriber.
type EventPoller interface {
	PollEvents(ctx context.Context, since int64, limit int) ([]RawEvent, error)
}

// RawEvent is a workspace event as returned by GET /events.
type RawEvent struct {
	ID        int64
	Type      string
	Payload   json.RawMessage
	CreatedAt time.Time
}

// ── SUBSCRIBER ────────────────────────────────────────────────────────────────

// Subscriber polls Nexus for workspace events and fires matching workflows.
type Subscriber struct {
	nexusAddr  string
	httpClient *http.Client
	registry   *Registry
	executor   *workflow.Executor
	logger     *log.Logger
	lastID     int64
	sem         chan struct{} // bounded semaphore — FG-Fix-02
	serviceToken string       // ADR-008: X-Service-Token for Nexus calls
}

// NewSubscriber creates a Subscriber.
func NewSubscriber(
	nexusAddr    string,
	registry     *Registry,
	executor     *workflow.Executor,
	logger       *log.Logger,
	serviceToken string, // ADR-008
) *Subscriber {
	return &Subscriber{
		nexusAddr:    nexusAddr,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		registry:     registry,
		executor:     executor,
		logger:       logger,
		sem:          make(chan struct{}, maxConcurrentWorkflows),
		serviceToken: serviceToken,
	}
}

// Run starts the polling loop and blocks until ctx is cancelled.
func (s *Subscriber) Run(ctx context.Context) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	s.logger.Printf("trigger subscriber started (polling Nexus every %s)", pollInterval)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.poll(ctx)
		}
	}
}

// poll fetches recent events from Nexus and dispatches matching triggers.
func (s *Subscriber) poll(ctx context.Context) {
	url := fmt.Sprintf("%s/events?limit=%d", s.nexusAddr, pollEventLimit)
	if s.lastID > 0 {
		url += "&since=" + strconv.FormatInt(s.lastID, 10)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}
	if s.serviceToken != "" {
		req.Header.Set(canon.ServiceTokenHeader, s.serviceToken) // ADR-008
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return // Nexus temporarily unavailable — retry next tick
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Printf("WARNING: Nexus poll returned HTTP %d — will retry next tick", resp.StatusCode)
		return
	}

	var envelope struct {
		OK   bool `json:"ok"`
		Data []struct {
			ID        int64           `json:"id"`
			Type      string          `json:"type"`
			Payload   json.RawMessage `json:"payload"`
			CreatedAt time.Time       `json:"created_at"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return
	}
	if !envelope.OK {
		return
	}

	for _, e := range envelope.Data {
		if e.ID > s.lastID {
			s.lastID = e.ID
		}

		// Only process workspace topics (ADR-007).
		if !SupportedEvents[e.Type] {
			continue
		}

		payload := s.extractPayload(e.Type, e.Payload)
		s.dispatch(ctx, e.Type, payload)
	}
}

// dispatch finds matching triggers and fires each workflow asynchronously.
func (s *Subscriber) dispatch(
	ctx context.Context,
	event string,
	payload WorkspaceEventPayload,
) {
	triggers, err := s.registry.MatchingTriggers(event, payload)
	if err != nil {
		s.logger.Printf("WARNING: trigger registry error for %s: %v", event, err)
		return
	}

	for _, t := range triggers {
		t := t // capture for goroutine
		// Try to acquire a semaphore slot. If all slots are taken, drop this
		// trigger rather than spawning an unbounded goroutine (FG-Fix-02).
		select {
		case s.sem <- struct{}{}:
			// slot acquired — proceed
		default:
			s.logger.Printf("WARNING: trigger %s dropped — max concurrent workflows (%d) reached",
				t.ID, maxConcurrentWorkflows)
			continue
		}
		go func() {
			defer func() { <-s.sem }() // release slot on completion
			baseCtx := command.CommandContext{
				WorkspaceRoot:   payload.Path,
				RequestingAgent: "trigger",
				Timestamp:       time.Now().UTC(),
			}
			result, err := s.executor.Run(ctx, t.WorkflowID, baseCtx)
			if err != nil {
				s.logger.Printf("trigger %s: workflow %s error: %v",
					t.ID, t.WorkflowID, err)
				return
			}
			if result.Success {
				s.logger.Printf("trigger %s: workflow %s completed ✓ (%d steps)",
					t.ID, t.WorkflowID, result.StepsDone)
			} else {
				s.logger.Printf("trigger %s: workflow %s failed at step %d: %s",
					t.ID, t.WorkflowID, result.StepsDone, result.Error)
			}
		}()
	}
}

// extractPayload parses the event payload into a WorkspaceEventPayload.
// Returns an empty payload on parse failure — triggers with no filter still fire.
func (s *Subscriber) extractPayload(eventType string, raw json.RawMessage) WorkspaceEventPayload {
	switch eventType {
	case nexusevents.TopicWorkspaceFileCreated,
		nexusevents.TopicWorkspaceFileModified,
		nexusevents.TopicWorkspaceFileDeleted:
		var p nexusevents.WorkspaceFilePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return WorkspaceEventPayload{}
		}
		return WorkspaceEventPayload{
			Path:      p.Path,
			Extension: p.Extension,
		}
	}
	return WorkspaceEventPayload{}
}
