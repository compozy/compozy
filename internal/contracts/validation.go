package contracts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

const maxVerdictIssues = 25

// ValidateSchemaDefinition validates one authored JSON Schema object without treating it as a result contract shorthand.
func ValidateSchemaDefinition(schema json.RawMessage) error {
	trimmed := bytes.TrimSpace(schema)
	if len(trimmed) == 0 {
		return errors.New("schema object is required")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return fmt.Errorf("parse schema definition: %w", err)
	}
	if object == nil {
		return errors.New("schema must be a JSON object")
	}
	if _, err := compileSchema(Contract{Schema: cloneRaw(trimmed)}); err != nil {
		return err
	}
	return nil
}

func validatePayload(schema *jsonschema.Schema, raw json.RawMessage) Verdict {
	sanitized := sanitizeRawBytes(raw)
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(sanitized))
	if err != nil {
		return invalidVerdict([]ValidationIssue{{Path: "$", Message: sanitizedMessage(err.Error())}})
	}
	validationErr := schema.Validate(instance)
	if validationErr == nil {
		return Verdict{Valid: true}
	}
	if inner, ok := singleKeyWrapper(instance); ok {
		if err := schema.Validate(inner); err == nil {
			return Verdict{Valid: true, Unwrapped: true}
		}
	}
	return invalidVerdict(validationIssues(validationErr))
}

func invalidVerdict(issues []ValidationIssue) Verdict {
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Path == issues[j].Path {
			return issues[i].Message < issues[j].Message
		}
		return issues[i].Path < issues[j].Path
	})
	if len(issues) > maxVerdictIssues {
		issues = issues[:maxVerdictIssues]
	}
	return Verdict{Valid: false, Issues: issues}
}

func singleKeyWrapper(value any) (any, bool) {
	object, ok := value.(map[string]any)
	if !ok || len(object) != 1 {
		return nil, false
	}
	for _, child := range object {
		if _, ok := child.(map[string]any); ok {
			return child, true
		}
	}
	return nil, false
}

func validationIssues(err error) []ValidationIssue {
	validationErr, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return []ValidationIssue{{Path: "$", Message: sanitizedMessage(err.Error())}}
	}
	issues := make([]ValidationIssue, 0, len(validationErr.Causes))
	collectValidationIssues(validationErr, &issues)
	if len(issues) == 0 {
		issues = append(issues, ValidationIssue{
			Path:    instancePath(validationErr.InstanceLocation),
			Message: sanitizedMessage(validationErr.Error()),
		})
	}
	return issues
}

func collectValidationIssues(err *jsonschema.ValidationError, issues *[]ValidationIssue) {
	if required, ok := err.ErrorKind.(*kind.Required); ok {
		for _, missing := range required.Missing {
			location := append(append([]string(nil), err.InstanceLocation...), missing)
			*issues = append(*issues, ValidationIssue{
				Path:    instancePath(location),
				Message: sanitizedMessage(err.Error()),
			})
		}
		return
	}
	if len(err.Causes) == 0 {
		*issues = append(*issues, ValidationIssue{
			Path:    instancePath(err.InstanceLocation),
			Message: sanitizedMessage(err.Error()),
		})
		return
	}
	for _, cause := range err.Causes {
		collectValidationIssues(cause, issues)
	}
}

func instancePath(tokens []string) string {
	var path strings.Builder
	path.WriteByte('$')
	for _, token := range tokens {
		if isDecimal(token) {
			fmt.Fprintf(&path, "[%s]", token)
			continue
		}
		path.WriteByte('.')
		path.WriteString(token)
	}
	return path.String()
}

func isDecimal(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// BuildRepairPrompt renders bounded validator output without repeating a schema.
func BuildRepairPrompt(issues []ValidationIssue) string {
	const issueLimit = 10
	var prompt strings.Builder
	prompt.WriteString("Your result did not satisfy the output contract:\n")
	rendered := min(len(issues), issueLimit)
	for _, issue := range issues[:rendered] {
		fmt.Fprintf(&prompt, "- %s: %s\n", issue.Path, sanitizedMessage(issue.Message))
	}
	if remaining := len(issues) - rendered; remaining > 0 {
		fmt.Fprintf(&prompt, "(+%d more)\n", remaining)
	}
	prompt.WriteString("Return one JSON object that fixes these issues.")
	return prompt.String()
}

func sanitizedMessage(message string) string {
	clean, _, reject := SanitizeText(message)
	if reject {
		return "validator detail was removed because it contained only secret material"
	}
	return clean
}

// ValidateWaitPayload validates a persisted wait response through the shared engine.
func ValidateWaitPayload(expect json.RawMessage, payload json.RawMessage) error {
	if len(bytes.TrimSpace(payload)) == 0 || !json.Valid(payload) {
		return errors.New("wait payload must be valid JSON")
	}
	if len(bytes.TrimSpace(expect)) == 0 {
		return nil
	}
	canonical, err := normalizeSchema(expect)
	if err != nil {
		return fmt.Errorf("decode wait schema: %w", err)
	}
	compiled, err := compileSchema(Contract{Schema: canonical})
	if err != nil {
		return fmt.Errorf("compile wait schema: %w", err)
	}
	verdict := validatePayload(compiled, payload)
	if verdict.Valid {
		return nil
	}
	return fmt.Errorf("wait payload does not match expect: %s", BuildRepairPrompt(verdict.Issues))
}

// ValidateSchema validates a payload directly against an authored schema.
// Consumers without a durable registry digest use this adapter so they still
// share canonical normalization and validation behavior.
func ValidateSchema(schema json.RawMessage, payload json.RawMessage) (Verdict, error) {
	canonical, err := normalizeSchema(schema)
	if err != nil {
		return Verdict{}, newError(CodeExpectInvalid, FaultContract, err.Error(), err)
	}
	compiled, err := compileSchema(Contract{Schema: canonical})
	if err != nil {
		return Verdict{}, newError(CodeContractCompile, FaultContract, err.Error(), err)
	}
	return validatePayload(compiled, payload), nil
}

// ValidateSchemaFragment validates against a nested schema fragment. Unlike a
// top-level contract, a fragment may describe a scalar or array value.
func ValidateSchemaFragment(schema json.RawMessage, payload json.RawMessage) (Verdict, error) {
	compiled, err := compileSchema(Contract{Schema: cloneRaw(schema)})
	if err != nil {
		return Verdict{}, newError(CodeContractCompile, FaultContract, err.Error(), err)
	}
	return validatePayload(compiled, payload), nil
}
