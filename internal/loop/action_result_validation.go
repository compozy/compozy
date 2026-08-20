package loop

import (
	"bytes"
	"fmt"

	"github.com/compozy/compozy/internal/task"
)

// ValidateActionRunResult revalidates the exact durable result against the
// output schema pinned into the task-run metadata before lease settlement.
func ValidateActionRunResult(run task.Run, result task.RunResult) error {
	if !run.IsLoopWorker() {
		return nil
	}
	meta, err := parseCoordinatorActionRunMetadata(run.Metadata)
	if err != nil {
		return err
	}
	if len(meta.OutputSchema) == 0 {
		return nil
	}
	if result.CoordinatorControl != nil {
		return actionInvalidOutputError(fmt.Errorf("run-agent result cannot contain coordinator control"))
	}
	payload := bytes.TrimSpace(result.Value)
	if len(payload) == 0 {
		return actionInvalidOutputError(fmt.Errorf("run-agent result is empty"))
	}
	_, err = ValidateActionStructured(meta.OutputSchema, ActionPromptResult{Structured: payload})
	return err
}
