package loop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const jsonSchemaEnumKey = "enum"

func validateRunAgentStructured(schema dsl.Schema, result ActionPromptResult) (json.RawMessage, error) {
	return ValidateActionStructured(schema, result)
}

// ValidateActionStructured applies the Loop-owned generation-output schema validator.
// Child action executors use this seam instead of owning a second schema implementation.
// Free-text turns are scanned newest-first and the first candidate object that
// satisfies the schema wins, so JSON quoted mid-turn cannot shadow the answer.
func ValidateActionStructured(schema dsl.Schema, result ActionPromptResult) (json.RawMessage, error) {
	if len(schema) == 0 {
		if len(bytes.TrimSpace(result.Structured)) > 0 {
			return cloneRawMessage(result.Structured), nil
		}
		return nil, nil
	}
	if len(bytes.TrimSpace(result.Structured)) > 0 {
		if !json.Valid(result.Structured) {
			return nil, actionSchemaInvalidError(errors.New("structured result is not valid JSON"))
		}
		raw := cloneRawMessage(result.Structured)
		if err := validateJSONSchema(schema, raw); err != nil {
			return nil, actionSchemaInvalidError(err)
		}
		return raw, nil
	}
	candidates := extractJSONObjectCandidates(result.Text)
	if len(candidates) == 0 {
		return nil, actionSchemaInvalidError(errors.New("no JSON object found"))
	}
	var newestErr error
	for _, candidate := range slices.Backward(candidates) {
		err := validateJSONSchema(schema, candidate)
		if err == nil {
			return candidate, nil
		}
		if newestErr == nil {
			newestErr = err
		}
	}
	return nil, actionSchemaInvalidError(newestErr)
}

func actionSchemaInvalidError(err error) error {
	return newSafeActionFailureError(
		reasonError(ReasonCodeActionSchemaInvalid, errors.Join(ErrActionSchemaInvalid, err), nil),
		NewActionFailure(
			string(ReasonCodeActionSchemaInvalid),
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
	schemaDoc, err := normalizeLoopSchema(schema)
	if err != nil {
		return err
	}
	schemaData, err := json.Marshal(schemaDoc)
	if err != nil {
		return fmt.Errorf("marshal output schema: %w", err)
	}
	schemaValue, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaData))
	if err != nil {
		return fmt.Errorf("parse output schema: %w", err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("parse structured output: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("output_schema.json", schemaValue); err != nil {
		return fmt.Errorf("add output schema resource: %w", err)
	}
	compiled, err := compiler.Compile("output_schema.json")
	if err != nil {
		return fmt.Errorf("compile output schema: %w", err)
	}
	if err := compiled.Validate(instance); err != nil {
		return fmt.Errorf("validate structured output: %w", err)
	}
	return nil
}

// ValidateWaitPayload validates one admitted wait payload against its persisted schema.
func ValidateWaitPayload(expect json.RawMessage, payload json.RawMessage) error {
	if len(bytes.TrimSpace(payload)) == 0 || !json.Valid(payload) {
		return fmt.Errorf("%w: wait payload must be valid JSON", ErrValidation)
	}
	if len(bytes.TrimSpace(expect)) == 0 {
		return nil
	}
	var schema dsl.Schema
	if err := json.Unmarshal(expect, &schema); err != nil {
		return fmt.Errorf("%w: decode wait schema: %v", ErrValidation, err)
	}
	if err := validateJSONSchema(schema, payload); err != nil {
		return fmt.Errorf("%w: wait payload does not match expect: %v", ErrValidation, err)
	}
	return nil
}

func normalizeLoopSchema(schema dsl.Schema) (map[string]any, error) {
	normalized, err := normalizeJSONValue(map[string]any(schema))
	if err != nil {
		return nil, err
	}
	schemaMap, ok := normalized.(map[string]any)
	if !ok {
		return nil, errors.New("output schema must normalize to object")
	}
	if isFullJSONSchema(schemaMap) {
		return schemaMap, nil
	}
	properties := make(map[string]any, len(schemaMap))
	required := make([]string, 0, len(schemaMap))
	for key, value := range schemaMap {
		properties[key] = shorthandPropertySchema(value)
		required = append(required, key)
	}
	sort.Strings(required)
	return map[string]any{
		jsonSchemaTypeKey:       jsonSchemaObjectType,
		jsonSchemaPropertiesKey: properties,
		jsonSchemaRequiredKey:   required,
	}, nil
}

func isFullJSONSchema(schema map[string]any) bool {
	fullKeys := map[string]struct{}{
		"$defs":                           {},
		"$id":                             {},
		"$schema":                         {},
		jsonSchemaAdditionalPropertiesKey: {},
		jsonSchemaAllOfKey:                {},
		jsonSchemaAnyOfKey:                {},
		"contains":                        {},
		"contentEncoding":                 {},
		"contentMediaType":                {},
		"const":                           {},
		jsonSchemaDefaultKey:              {},
		"dependentRequired":               {},
		jsonSchemaDependentSchemasKey:     {},
		"description":                     {},
		jsonSchemaEnumKey:                 {},
		"examples":                        {},
		"exclusiveMaximum":                {},
		"exclusiveMinimum":                {},
		"format":                          {},
		"if":                              {},
		jsonSchemaItemsKey:                {},
		"maxItems":                        {},
		"maxLength":                       {},
		"maxProperties":                   {},
		"maximum":                         {},
		"minItems":                        {},
		"minLength":                       {},
		"minProperties":                   {},
		"minimum":                         {},
		"multipleOf":                      {},
		"not":                             {},
		jsonSchemaOneOfKey:                {},
		"pattern":                         {},
		"prefixItems":                     {},
		jsonSchemaPropertiesKey:           {},
		"propertyNames":                   {},
		jsonSchemaRequiredKey:             {},
		jsonSchemaThenKey:                 {},
		jsonSchemaTitleKey:                {},
		jsonSchemaTypeKey:                 {},
		"unevaluatedProperties":           {},
		jsonSchemaEntityKindKey:           {},
	}
	hasKeyword := false
	for key := range schema {
		if _, ok := fullKeys[key]; !ok {
			return false
		}
		hasKeyword = true
	}
	return hasKeyword
}

func shorthandPropertySchema(value any) any {
	switch typed := value.(type) {
	case string:
		return map[string]any{jsonSchemaTypeKey: typed}
	case map[string]any:
		if isFullJSONSchema(typed) {
			return typed
		}
		nested := make(map[string]any, len(typed))
		for key, child := range typed {
			nested[key] = shorthandPropertySchema(child)
		}
		return map[string]any{
			jsonSchemaTypeKey:       jsonSchemaObjectType,
			jsonSchemaPropertiesKey: nested,
		}
	default:
		return value
	}
}

// runAgentPromptWithOutputContract appends the authored output schema so the
// agent knows the exact terminal JSON shape before its first attempt.
func runAgentPromptWithOutputContract(prompt string, schema dsl.Schema) (string, error) {
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
	candidates := extractJSONObjectCandidates(text)
	if len(candidates) == 0 {
		return nil, errors.New("no JSON object found")
	}
	return candidates[0], nil
}

// extractJSONObjectCandidates returns every distinct top-level JSON object in
// the turn text, in order of appearance.
func extractJSONObjectCandidates(text string) []json.RawMessage {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	collected := make([]jsonObjectCandidate, 0, 6)
	if raw, ok := validJSONObject(trimmed); ok {
		collected = append(collected, jsonObjectCandidate{raw: raw, offset: strings.Index(text, trimmed)})
	}
	collected = append(collected, extractFencedJSONObjects(text)...)
	collected = append(collected, extractBalancedJSONObjects(text)...)
	sort.SliceStable(collected, func(left, right int) bool {
		return collected[left].offset < collected[right].offset
	})
	seen := make(map[string]struct{})
	candidates := make([]json.RawMessage, 0, len(collected))
	for _, candidate := range collected {
		key := string(candidate.raw)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		candidates = append(candidates, candidate.raw)
	}
	return candidates
}

type jsonObjectCandidate struct {
	raw    json.RawMessage
	offset int
}

func extractFencedJSONObjects(text string) []jsonObjectCandidate {
	objects := make([]jsonObjectCandidate, 0, 2)
	start := 0
	for {
		open := strings.Index(text[start:], "```")
		if open < 0 {
			return objects
		}
		bodyStart := start + open + len("```")
		closeRel := strings.Index(text[bodyStart:], "```")
		if closeRel < 0 {
			return objects
		}
		body := text[bodyStart : bodyStart+closeRel]
		bodyOffset := bodyStart
		if newline := strings.IndexByte(body, '\n'); newline >= 0 {
			body = body[newline+1:]
			bodyOffset += newline + 1
		}
		trimmedBody := strings.TrimSpace(body)
		if raw, ok := validJSONObject(trimmedBody); ok {
			objects = append(objects, jsonObjectCandidate{
				raw: raw, offset: bodyOffset + strings.Index(body, trimmedBody),
			})
		}
		start = bodyStart + closeRel + len("```")
	}
}

func extractBalancedJSONObjects(text string) []jsonObjectCandidate {
	objects := make([]jsonObjectCandidate, 0, 2)
	cursor := 0
	for cursor < len(text) {
		offset := strings.IndexByte(text[cursor:], '{')
		if offset < 0 {
			return objects
		}
		start := cursor + offset
		raw, end, ok := balancedJSONObjectAt(text, start)
		if ok {
			objects = append(objects, jsonObjectCandidate{raw: raw, offset: start})
			cursor = end
			continue
		}
		cursor = start + 1
	}
	return objects
}

func balancedJSONObjectAt(text string, start int) (json.RawMessage, int, bool) {
	depth := 0
	inString := false
	escaped := false
	for idx := start; idx < len(text); idx++ {
		char := text[idx]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch char {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				raw, ok := validJSONObject(strings.TrimSpace(text[start : idx+1]))
				return raw, idx + 1, ok
			}
		}
	}
	return nil, len(text), false
}

func validJSONObject(candidate string) (json.RawMessage, bool) {
	if candidate == "" || candidate[0] != '{' || !json.Valid([]byte(candidate)) {
		return nil, false
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(candidate), &decoded); err != nil {
		return nil, false
	}
	if decoded == nil {
		return nil, false
	}
	return json.RawMessage(candidate), true
}
