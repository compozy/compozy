package loop

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GenerationResultKind identifies how a generation result participates in control flow and templates.
type GenerationResultKind string

const (
	GenerationResultPayload            GenerationResultKind = "payload"
	GenerationResultFailure            GenerationResultKind = "failure"
	GenerationResultControlValue       GenerationResultKind = "control_value"
	GenerationResultBranchTrue         GenerationResultKind = "branch_true"
	GenerationResultBranchFalse        GenerationResultKind = "branch_false"
	GenerationResultSkipped            GenerationResultKind = "skipped"
	GenerationResultRouteSelected      GenerationResultKind = "route_selected"
	GenerationResultRouteNotTaken      GenerationResultKind = "route_not_taken"
	GenerationResultErrorRouted        GenerationResultKind = "error_routed"
	GenerationResultFailureAbsorbed    GenerationResultKind = "failure_absorbed"
	GenerationResultWaitExpiryRouted   GenerationResultKind = "wait_expiry_routed"
	GenerationResultReviewRejectRouted GenerationResultKind = "review_reject_routed"
	GenerationResultStrategyCanceled   GenerationResultKind = "strategy_canceled"
	GenerationResultStrategyNotStarted GenerationResultKind = "strategy_not_started"
)

// GenerationResultRef is the explicit shape persisted in loop_generation_outputs.output_ref.
type GenerationResultRef struct {
	Kind       GenerationResultKind `json:"kind"`
	SchemaRef  string               `json:"schema_ref,omitempty"`
	PayloadRef string               `json:"payload_ref,omitempty"`
}

func (r GenerationResultRef) validate() error {
	switch r.Kind {
	case GenerationResultPayload,
		GenerationResultFailure,
		GenerationResultControlValue,
		GenerationResultBranchTrue,
		GenerationResultBranchFalse,
		GenerationResultSkipped,
		GenerationResultRouteSelected,
		GenerationResultRouteNotTaken,
		GenerationResultErrorRouted,
		GenerationResultFailureAbsorbed,
		GenerationResultWaitExpiryRouted,
		GenerationResultReviewRejectRouted,
		GenerationResultStrategyCanceled,
		GenerationResultStrategyNotStarted:
		return nil
	default:
		return fmt.Errorf("%w: generation result kind %q is invalid", ErrValidation, r.Kind)
	}
}

// EncodeGenerationResultRef encodes the only durable generation result shape.
func EncodeGenerationResultRef(result GenerationResultRef) (string, error) {
	result.SchemaRef = strings.TrimSpace(result.SchemaRef)
	result.PayloadRef = strings.TrimSpace(result.PayloadRef)
	if err := result.validate(); err != nil {
		return "", err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("loop: encode generation result: %w", err)
	}
	return string(raw), nil
}

// DecodeGenerationResultRef decodes an explicit result. Bare legacy sentinels
// and untagged payloads are rejected instead of being guessed by readers.
func DecodeGenerationResultRef(raw string) (GenerationResultRef, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return GenerationResultRef{}, nil
	}
	var result GenerationResultRef
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		return GenerationResultRef{}, fmt.Errorf("%w: decode generation result: %v", ErrValidation, err)
	}
	if err := result.validate(); err != nil {
		return GenerationResultRef{}, err
	}
	result.SchemaRef = strings.TrimSpace(result.SchemaRef)
	result.PayloadRef = strings.TrimSpace(result.PayloadRef)
	return result, nil
}

func generationResultForRef(ref string) GenerationResultRef {
	trimmed := strings.TrimSpace(ref)
	result := GenerationResultRef{Kind: GenerationResultPayload, PayloadRef: trimmed}
	switch {
	case trimmed == branchTrueOutputRef:
		result.Kind = GenerationResultBranchTrue
	case trimmed == branchFalseOutputRef:
		result.Kind = GenerationResultBranchFalse
	case trimmed == branchSkippedOutputRef:
		result.Kind = GenerationResultSkipped
	case strings.HasPrefix(trimmed, "route:"):
		result.Kind = GenerationResultRouteSelected
	case strings.HasPrefix(trimmed, routeNotTakenOutputRefPrefix):
		result.Kind = GenerationResultRouteNotTaken
	case strings.HasPrefix(trimmed, errorRoutedOutputRefPrefix):
		result.Kind = GenerationResultErrorRouted
	case trimmed == failureAbsorbedOutputRef:
		result.Kind = GenerationResultFailureAbsorbed
	case strings.HasPrefix(trimmed, waitExpiryRouteOutputRefPrefix):
		result.Kind = GenerationResultWaitExpiryRouted
	case strings.HasPrefix(trimmed, reviewRejectedRouteOutputRefPrefix):
		result.Kind = GenerationResultReviewRejectRouted
	case trimmed == strategyCanceledReasonCode:
		result.Kind = GenerationResultStrategyCanceled
	case trimmed == strategyNeverStartedReasonCode:
		result.Kind = GenerationResultStrategyNotStarted
	case actionFailureFromOutputRef(trimmed).Kind == actionFailureKind:
		result.Kind = GenerationResultFailure
	case trimmed != "" && !json.Valid([]byte(trimmed)) && !OutputRefLooksContentAddressed(trimmed):
		result.Kind = GenerationResultControlValue
	}
	return result
}

// EncodeGenerationResultForRef classifies a producer-owned ref once at the
// write boundary and returns the explicit durable result envelope.
func EncodeGenerationResultForRef(ref string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", nil
	}
	return EncodeGenerationResultRef(generationResultForRef(ref))
}

func encodeGenerationOutputResult(output GenerationOutput) (string, error) {
	if output.ResultKind == "" && strings.TrimSpace(output.OutputRef) == "" {
		return "", nil
	}
	result := GenerationResultRef{
		Kind:       output.ResultKind,
		SchemaRef:  output.SchemaRef,
		PayloadRef: output.OutputRef,
	}
	if result.Kind == "" {
		result = generationResultForRef(output.OutputRef)
		result.SchemaRef = output.SchemaRef
	}
	return EncodeGenerationResultRef(result)
}

// EncodedResult returns the explicit durable envelope for this output.
func (output GenerationOutput) EncodedResult() (string, error) {
	return encodeGenerationOutputResult(output)
}

func generationResultValue(result GenerationResultRef, hydrated json.RawMessage) any {
	if result.Kind == "" {
		result = generationResultForRef(result.PayloadRef)
	}
	switch result.Kind {
	case GenerationResultPayload, GenerationResultFailure:
		payload := []byte(result.PayloadRef)
		if len(hydrated) > 0 {
			payload = hydrated
		}
		var value any
		if err := decodeSingleJSONValue(strings.NewReader(string(payload)), &value); err != nil {
			return nil
		}
		return value
	case GenerationResultControlValue,
		GenerationResultBranchTrue,
		GenerationResultBranchFalse,
		GenerationResultRouteSelected:
		return result.PayloadRef
	default:
		return nil
	}
}

func generationOutputHasKind(output GenerationOutput, kinds ...GenerationResultKind) bool {
	actual := output.ResultKind
	if actual == "" {
		actual = generationResultForRef(output.OutputRef).Kind
	}
	for _, kind := range kinds {
		if actual == kind {
			return true
		}
	}
	return false
}

func generationOutputRepresentsAbsentValue(output GenerationOutput) bool {
	return generationOutputHasKind(
		output,
		GenerationResultSkipped,
		GenerationResultRouteNotTaken,
		GenerationResultErrorRouted,
		GenerationResultFailureAbsorbed,
		GenerationResultWaitExpiryRouted,
		GenerationResultReviewRejectRouted,
		GenerationResultStrategyCanceled,
		GenerationResultStrategyNotStarted,
	)
}
