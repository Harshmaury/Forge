// @forge-project: forge
// @forge-path: internal/workflow/model.go
// Package workflow defines types for workflow creation and execution.
// Workflows wrap Command objects — ADR-004 schema is unchanged.
package workflow

import (
	"fmt"
	"time"

	"github.com/Harshmaury/Forge/internal/executor/intent"
	"github.com/Harshmaury/Forge/internal/store"
)

// CreateWorkflowRequest is the HTTP body for POST /workflows.
type CreateWorkflowRequest struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Steps       []StepInput `json:"steps"`
}

// StepInput is one step in a CreateWorkflowRequest.
type StepInput struct {
	Intent     string            `json:"intent"`
	Target     string            `json:"target"`
	Parameters map[string]string `json:"parameters"`
}

// WorkflowRunResult is the result of running a complete workflow.
type WorkflowRunResult struct {
	WorkflowID   string        `json:"workflow_id"`
	WorkflowName string        `json:"workflow_name"`
	Success      bool          `json:"success"`
	StepsTotal   int           `json:"steps_total"`
	StepsDone    int           `json:"steps_done"`
	StepResults  []*StepResult `json:"step_results"`
	Duration     time.Duration `json:"duration"`
	Error        string        `json:"error,omitempty"`
}

// StepResult is the result of one step in a workflow execution.
type StepResult struct {
	Position int            `json:"position"`
	Intent   string         `json:"intent"`
	Target   string         `json:"target"`
	Result   *intent.Result `json:"result"`
}

// Validate checks that a CreateWorkflowRequest has all required fields.
func (r *CreateWorkflowRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(r.Steps) == 0 {
		return fmt.Errorf("at least one step is required")
	}
	for i, s := range r.Steps {
		if s.Intent == "" {
			return fmt.Errorf("step %d: intent is required", i+1)
		}
		if s.Target == "" {
			return fmt.Errorf("step %d: target is required", i+1)
		}
	}
	return nil
}

// ToStoreSteps converts request steps to store.WorkflowStep records.
func (r *CreateWorkflowRequest) ToStoreSteps(workflowID string) []*store.WorkflowStep {
	steps := make([]*store.WorkflowStep, len(r.Steps))
	for i, s := range r.Steps {
		params := s.Parameters
		if params == nil {
			params = map[string]string{}
		}
		steps[i] = &store.WorkflowStep{
			WorkflowID: workflowID,
			Position:   i + 1,
			Intent:     s.Intent,
			Target:     s.Target,
			Parameters: params,
		}
	}
	return steps
}
