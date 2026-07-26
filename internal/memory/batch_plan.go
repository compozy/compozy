package memory

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/memory/scan"
)

// BatchAction identifies one staged mutation of a Memory v2 document body.
type BatchAction string

const (
	BatchActionAdd     BatchAction = "add"
	BatchActionReplace BatchAction = "replace"
	BatchActionRemove  BatchAction = "remove"
)

// BatchOperation is one closed add, replace, or remove step in a document batch.
type BatchOperation struct {
	Action  BatchAction `json:"action"`
	Content string      `json:"content,omitempty"`
	OldText string      `json:"old_text,omitempty"`
}

// BatchOperationOutcome reports how one operation participated in the final commit.
type BatchOperationOutcome struct {
	Index   int         `json:"index"`
	Action  BatchAction `json:"action"`
	Changed bool        `json:"changed"`
	Status  string      `json:"status"`
}

type batchPlan struct {
	body     string
	outcomes []BatchOperationOutcome
}

func planMemoryBatch(initialBody string, operations []BatchOperation) (batchPlan, error) {
	if len(operations) == 0 {
		return batchPlan{}, batchValidationError("operations list is empty")
	}

	working := strings.TrimSpace(initialBody)
	outcomes := make([]BatchOperationOutcome, 0, len(operations))
	for index, operation := range operations {
		op, err := normalizeBatchOperation(index, operation)
		if err != nil {
			return batchPlan{}, err
		}
		before := working
		working, err = applyBatchOperation(working, index, op)
		if err != nil {
			return batchPlan{}, err
		}
		outcomes = append(outcomes, BatchOperationOutcome{
			Index:   index,
			Action:  op.Action,
			Changed: working != before,
		})
	}

	return batchPlan{
		body:     strings.TrimSpace(working),
		outcomes: outcomes,
	}, nil
}

func normalizeBatchOperation(index int, operation BatchOperation) (BatchOperation, error) {
	normalized := BatchOperation{
		Action:  BatchAction(strings.ToLower(strings.TrimSpace(string(operation.Action)))),
		Content: strings.TrimSpace(operation.Content),
		OldText: strings.TrimSpace(operation.OldText),
	}
	position := index + 1
	switch normalized.Action {
	case BatchActionAdd:
		if normalized.Content == "" {
			return BatchOperation{}, batchValidationError("operation %d (add): content is required", position)
		}
		if normalized.OldText != "" {
			return BatchOperation{}, batchValidationError("operation %d (add): old_text is not allowed", position)
		}
	case BatchActionReplace:
		if normalized.OldText == "" {
			return BatchOperation{}, batchValidationError("operation %d (replace): old_text is required", position)
		}
		if normalized.Content == "" {
			return BatchOperation{}, batchValidationError(
				"operation %d (replace): content is required; use remove to delete text",
				position,
			)
		}
	case BatchActionRemove:
		if normalized.OldText == "" {
			return BatchOperation{}, batchValidationError("operation %d (remove): old_text is required", position)
		}
		if normalized.Content != "" {
			return BatchOperation{}, batchValidationError("operation %d (remove): content is not allowed", position)
		}
	default:
		return BatchOperation{}, batchValidationError(
			"operation %d: action must be add, replace, or remove",
			position,
		)
	}
	if normalized.Action != BatchActionRemove {
		result := scan.Content(normalized.Content)
		if result.Rejected() {
			return BatchOperation{}, batchValidationError("operation %d: %s", position, result.Reason())
		}
	}
	return normalized, nil
}

func applyBatchOperation(body string, index int, operation BatchOperation) (string, error) {
	switch operation.Action {
	case BatchActionAdd:
		if batchBodyContainsBlock(body, operation.Content) {
			return body, nil
		}
		if body == "" {
			return operation.Content, nil
		}
		return body + "\n\n" + operation.Content, nil
	case BatchActionReplace, BatchActionRemove:
		matches := strings.Count(body, operation.OldText)
		if matches != 1 {
			return "", batchValidationError(
				"operation %d (%s): old_text matched %d occurrences; expected exactly 1; no operations were applied",
				index+1,
				operation.Action,
				matches,
			)
		}
		replacement := operation.Content
		return strings.TrimSpace(strings.Replace(body, operation.OldText, replacement, 1)), nil
	default:
		return "", batchValidationError("operation %d: unsupported action %q", index+1, operation.Action)
	}
}

func batchBodyContainsBlock(body string, content string) bool {
	for block := range strings.SplitSeq(strings.TrimSpace(body), "\n\n") {
		if strings.TrimSpace(block) == content {
			return true
		}
	}
	return false
}

func batchValidationError(format string, args ...any) error {
	return fmt.Errorf("%w: memory batch: %s", ErrValidation, fmt.Sprintf(format, args...))
}
