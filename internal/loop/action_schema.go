package loop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/loop/dsl"
)

func validateRunAgentStructured(schema dsl.Schema, result ActionPromptResult) (json.RawMessage, error) {
	return ValidateActionStructured(schema, result)
}

// ValidateActionStructured applies the Loop-owned generation-output schema validator.
// Child action executors use this seam instead of owning a second schema implementation.
// Free-text turns are scanned newest-first and the first candidate object that
// satisfies the schema wins, so JSON quoted mid-turn cannot shadow the answer.
func ValidateActionStructured(schema dsl.Schema, result ActionPromptResult) (json.RawMessage, error) {
	return validateActionStructuredWith(schema, result, validateJSONSchema)
}

type actionSchemaValidator func(dsl.Schema, json.RawMessage) error

func validateActionStructuredWith(
	schema dsl.Schema,
	result ActionPromptResult,
	validate actionSchemaValidator,
) (json.RawMessage, error) {
	if len(schema) == 0 {
		if len(bytes.TrimSpace(result.Structured)) > 0 {
			return cloneRawMessage(result.Structured), nil
		}
		return nil, nil
	}
	if len(bytes.TrimSpace(result.Structured)) > 0 {
		if !json.Valid(result.Structured) {
			return nil, actionInvalidOutputError(errors.New("structured result is not valid JSON"))
		}
		raw := cloneRawMessage(result.Structured)
		if err := validate(schema, raw); err != nil {
			return nil, actionInvalidOutputError(err)
		}
		return raw, nil
	}
	candidates := contracts.ExtractCandidates(result.Text)
	if len(candidates) == 0 {
		return nil, actionInvalidOutputError(errors.New("no JSON object found"))
	}
	var newestErr error
	for _, candidate := range candidates {
		err := validate(schema, candidate)
		if err == nil {
			return candidate, nil
		}
		if newestErr == nil {
			newestErr = err
		}
	}
	return nil, actionInvalidOutputError(newestErr)
}

func actionInvalidOutputError(err error) error {
	return newSafeActionFailureError(
		reasonError(ReasonCodeInvalidOutput, errors.Join(ErrActionInvalidOutput, err), nil),
		NewActionFailure(
			string(ReasonCodeInvalidOutput),
			schemaInvalidCause(err),
			"Return one JSON object that satisfies every required output field, then retry the action.",
		),
	)
}

const schemaInvalidCauseLimit = 240

func schemaInvalidCause(err error) string {
	detail := ""
	if err != nil {
		detail = strings.TrimSpace(err.Error())
	}
	if detail == "" {
		return "The agent output did not satisfy the action output schema."
	}
	if len(detail) > schemaInvalidCauseLimit {
		detail = detail[:schemaInvalidCauseLimit] + "…"
	}
	return "The agent output did not satisfy the action output schema: " + detail
}

func structuredCandidate(result ActionPromptResult) (json.RawMessage, error) {
	if len(bytes.TrimSpace(result.Structured)) > 0 {
		if !json.Valid(result.Structured) {
			return nil, errors.New("structured result is not valid JSON")
		}
		return cloneRawMessage(result.Structured), nil
	}
	return extractJSONObject(result.Text)
}

// ActionStructuredCandidate returns the structured object supplied by one action prompt result.
func ActionStructuredCandidate(result ActionPromptResult) (json.RawMessage, error) {
	return structuredCandidate(result)
}

func validateJSONSchema(schema dsl.Schema, raw json.RawMessage) error {
	schemaData, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("marshal output schema: %w", err)
	}
	verdict, err := contracts.ValidateSchema(schemaData, raw)
	if err != nil {
		return err
	}
	if !verdict.Valid {
		return errors.New(contracts.BuildRepairPrompt(verdict.Issues))
	}
	return nil
}

// ValidateWaitPayload validates one admitted wait payload against its persisted schema.
func ValidateWaitPayload(expect json.RawMessage, payload json.RawMessage) error {
	if err := contracts.ValidateWaitPayload(expect, payload); err != nil {
		return fmt.Errorf("%w: %w", ErrValidation, err)
	}
	return nil
}

func normalizeLoopSchema(schema dsl.Schema) (map[string]any, error) {
	value, err := normalizeJSONValue(map[string]any(schema))
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	canonical, err := contracts.NormalizeSchema(raw)
	if err != nil {
		return nil, err
	}
	var normalized map[string]any
	if err := json.Unmarshal(canonical, &normalized); err != nil {
		return nil, fmt.Errorf("decode canonical output schema: %w", err)
	}
	return normalized, nil
}

// ActionPromptWithOutputContract appends the authored output schema so an
// action agent knows the exact terminal JSON shape before its first attempt.
func ActionPromptWithOutputContract(prompt string, schema dsl.Schema) (string, error) {
	if len(schema) == 0 {
		return prompt, nil
	}
	schemaDoc, err := normalizeLoopSchema(schema)
	if err != nil {
		return "", fmt.Errorf("normalize output contract schema: %w", err)
	}
	schemaData, err := json.Marshal(schemaDoc)
	if err != nil {
		return "", fmt.Errorf("marshal output contract schema: %w", err)
	}
	return fmt.Sprintf(
		"%s\n\n"+
			"Output contract:\n"+
			"End your final message with exactly one JSON object that satisfies this output_schema "+
			"(no other JSON object may follow it): %s",
		prompt,
		string(schemaData),
	), nil
}

func schemaRetryPrompt(prompt string, schema dsl.Schema, validationErr error) (string, error) {
	schemaDoc, err := normalizeLoopSchema(schema)
	if err != nil {
		return "", err
	}
	schemaData, err := json.Marshal(schemaDoc)
	if err != nil {
		return "", fmt.Errorf("marshal retry output schema: %w", err)
	}
	return fmt.Sprintf(
		"%s\n\n"+
			"Your previous response did not satisfy output_schema: %v\n"+
			"Return exactly one JSON object that satisfies this output_schema: %s",
		prompt,
		validationErr,
		string(schemaData),
	), nil
}

func extractJSONObject(text string) (json.RawMessage, error) {
	candidate, ok := contracts.ExtractCandidate(text)
	if !ok {
		return nil, errors.New("no JSON object found")
	}
	return candidate, nil
}
