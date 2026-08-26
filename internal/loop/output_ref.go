package loop

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/contracts"
)

const (
	// LoopOutputInlineLimitBytes is the largest loop node result kept inline in task_runs.
	LoopOutputInlineLimitBytes = 16 * 1024
)

// OutputPayloadRequiresRef reports whether a loop node result must be externalized.
func OutputPayloadRequiresRef(payload json.RawMessage) bool {
	return len(payload) > LoopOutputInlineLimitBytes
}

func generationOutputRefForPayload(
	payload json.RawMessage,
	outputBlobs *[]GenerationOutputBlob,
	at time.Time,
) (string, json.RawMessage, error) {
	if !json.Valid(payload) {
		return "", nil, fmt.Errorf("%w: generation output payload must be valid JSON", ErrValidation)
	}
	if !OutputPayloadRequiresRef(payload) {
		return string(payload), nil, nil
	}
	if outputBlobs == nil {
		return "", nil, fmt.Errorf("%w: output blob sink is required", ErrValidation)
	}
	ref := contracts.OutputRefForPayload(payload)
	*outputBlobs = append(*outputBlobs, GenerationOutputBlob{
		OutputRef: ref,
		Payload:   cloneRawMessage(payload),
		At:        at.UTC(),
	})
	return ref, cloneRawMessage(payload), nil
}
