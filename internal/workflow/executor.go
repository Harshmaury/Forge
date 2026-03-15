// @forge-project: forge
// @forge-path: internal/workflow/executor.go
// WorkflowExecutor runs a stored workflow step by step.
// Steps share CommandContext from the triggering command.
// First failure stops the chain — subsequent steps are not executed.
package workflow

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Harshmaury/Forge/internal/command"
	forgecontext "github.com/Harshmaury/Forge/internal/context"
	"github.com/Harshmaury/Forge/internal/executor"
	"github.com/Harshmaury/Forge/internal/store"
)

// Executor runs workflows step by step using the execution engine.
type Executor struct {
	store    store.Storer
	engine   *executor.Engine
	resolver *forgecontext.Resolver
	logger   *log.Logger
}

// NewExecutor creates a workflow Executor.
func NewExecutor(
	s store.Storer,
	e *executor.Engine,
	r *forgecontext.Resolver,
	logger *log.Logger,
) *Executor {
	return &Executor{store: s, engine: e, resolver: r, logger: logger}
}

// Run executes all steps of a workflow in order.
// Steps share the provided CommandContext.
// The first failed step stops execution.
func (ex *Executor) Run(
	ctx context.Context,
	workflowID string,
	baseContext command.CommandContext,
) (*WorkflowRunResult, error) {
	wf, err := ex.store.GetWorkflow(workflowID)
	if err != nil {
		return nil, fmt.Errorf("get workflow: %w", err)
	}
	if wf == nil {
		return nil, fmt.Errorf("workflow %q not found", workflowID)
	}

	steps, err := ex.store.GetSteps(workflowID)
	if err != nil {
		return nil, fmt.Errorf("get steps: %w", err)
	}

	start := time.Now()
	result := &WorkflowRunResult{
		WorkflowID:   wf.ID,
		WorkflowName: wf.Name,
		StepsTotal:   len(steps),
		StepResults:  make([]*StepResult, 0, len(steps)),
	}

	for _, step := range steps {
		cmd := ex.buildCommand(step, baseContext)

		// Enrich context per step.
		cmd = ex.resolver.ResolveContext(ctx, cmd)

		stepResult := ex.engine.Execute(ctx, cmd)
		result.StepResults = append(result.StepResults, &StepResult{
			Position: step.Position,
			Intent:   step.Intent,
			Target:   step.Target,
			Result:   stepResult,
		})
		result.StepsDone++

		if !stepResult.Success {
			result.Error = fmt.Sprintf("step %d (%s on %s) failed: %s",
				step.Position, step.Intent, step.Target, stepResult.Error)
			result.Duration = time.Since(start)
			ex.logger.Printf("workflow %s stopped at step %d: %s",
				workflowID, step.Position, result.Error)
			return result, nil
		}

		ex.logger.Printf("workflow %s step %d/%d (%s on %s) ✓",
			workflowID, step.Position, len(steps), step.Intent, step.Target)
	}

	result.Success = true
	result.Duration = time.Since(start)
	return result, nil
}

// buildCommand constructs a Command from a WorkflowStep and base context.
func (ex *Executor) buildCommand(
	step *store.WorkflowStep,
	baseCtx command.CommandContext,
) *command.Command {
	// Each step gets its own UUID via the base context timestamp offset.
	return &command.Command{
		ID:         fmt.Sprintf("wf-%s-step-%d", step.WorkflowID, step.Position),
		Intent:     step.Intent,
		Target:     step.Target,
		Parameters: step.Parameters,
		Context:    baseCtx,
	}
}
