package cli

import (
	"encoding/json"
	"strings"
	"unicode"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
	redactpkg "github.com/compozy/compozy/internal/redact"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/vault"
)

const (
	clientToolsAPIKeyKey = "api_key"
	clientToolsPromptKey = "prompt"
)

var toolResultDisplayJSONFields = []string{
	"",
	"authored_text",
	"body",
	agentCommandKey,
	"content",
	cliDescriptionKey,
	"detail",
	automationErrorKey,
	clientMessageKey,
	"output",
	loopPayloadKey,
	windowManagerPreviewKey,
	"raw",
	"raw_input",
	"raw_output",
	cliReasonKey,
	clientResultKey,
	"stderr",
	"stdout",
	"summary",
	sessionClarifyTextFlag,
	cliOutputTitleKey,
	"tool_input",
	"tool_result",
}

func sanitizeToolErrorResponse(response ToolErrorResponseRecord) ToolErrorResponseRecord {
	response.Error.Message = redactToolDiagnostic(response.Error.Message)
	response.Error.Details = contract.FilterToolOperatorFailureDetails(response.Error.Details)
	if response.Error.PartialResult != nil {
		response.Error.PartialResult = new(sanitizeToolResult(*response.Error.PartialResult))
	}
	return response
}

func sanitizeToolInvokeResponse(response ToolInvokeResponseRecord) ToolInvokeResponseRecord {
	response.Result = sanitizeToolResult(response.Result)
	return response
}

func sanitizeToolResult(result toolspkg.ToolResult) toolspkg.ToolResult {
	result.Preview = redactToolDiagnostic(result.Preview)
	result.Structured = redactToolRawJSON(result.Structured)
	result.Metadata = redactToolMetadata(result.Metadata)
	for i := range result.Content {
		result.Content[i].Text = redactToolDiagnostic(result.Content[i].Text)
		result.Content[i].Data = redactToolRawJSON(result.Content[i].Data)
		result.Content[i].Metadata = redactToolMetadata(result.Content[i].Metadata)
	}
	return result
}

func redactToolRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	if !json.Valid(raw) {
		redacted := redactToolDiagnostic(string(raw))
		if !json.Valid([]byte(redacted)) {
			return json.RawMessage(`"[REDACTED]"`)
		}
		return json.RawMessage(redacted)
	}
	engine := redactpkg.New(redactpkg.Options{Disabled: !redactpkg.Enabled()})
	return engine.RedactJSONWithProtection(
		raw,
		toolResultDisplayJSONFields,
		publicToolResultString,
		publicToolResultObject,
	)
}

func publicToolResultString(path []string, value string) bool {
	if len(path) == 0 {
		return false
	}
	key := path[len(path)-1]
	if strings.EqualFold(strings.TrimSpace(key), "ref") && vault.ValidateSecretRef(value) == nil {
		return true
	}
	return publicToolSchemaPattern(path)
}

func publicToolSchemaPattern(path []string) bool {
	if len(path) < 4 || (path[0] != "tool" && path[0] != "tools") || path[1] != "descriptor" {
		return false
	}
	if path[2] != "input_schema" && path[2] != "output_schema" {
		return false
	}
	// Only JSON Schema keyword positions describe validation syntax. Defaults,
	// examples, descriptions, and user properties named pattern remain data.
	path = path[3:]
	for len(path) > 1 {
		switch path[0] {
		case "properties", "patternProperties", "$defs", "definitions", "dependentSchemas":
			if len(path) < 3 {
				return false
			}
			path = path[2:]
		case "items", "prefixItems", "additionalItems", "additionalProperties", "contains",
			"propertyNames", "unevaluatedItems", "unevaluatedProperties", "allOf", "anyOf", "oneOf",
			"not", "if", "then", "else":
			path = path[1:]
		default:
			return false
		}
	}
	return path[0] == "pattern"
}

func publicToolResultObject(_ string, value any) bool {
	return toolspkg.IsPublicInputDeclaration(value)
}

func redactToolMetadata(metadata map[string]json.RawMessage) map[string]json.RawMessage {
	if len(metadata) == 0 {
		return metadata
	}
	redacted := make(map[string]json.RawMessage, len(metadata))
	for key, value := range metadata {
		if sensitiveToolFieldName(key) {
			redacted[key] = json.RawMessage(`"[REDACTED]"`)
			continue
		}
		redacted[key] = redactToolRawJSON(value)
	}
	return redacted
}

func sensitiveToolFieldName(key string) bool {
	parts := sensitiveToolFieldNameParts(key)
	normalized := strings.Join(parts, "_")
	const tokenField = "token"
	for _, marker := range []string{
		clientToolsAPIKeyKey,
		"authorization",
		"password",
		appDiagnosticBundleSecretTerm,
		"pkce",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	if len(parts) == 1 {
		return parts[0] == tokenField
	}
	if len(parts) == 0 || benignTokenMetric(parts) {
		return false
	}
	last := parts[len(parts)-1]
	return last == tokenField || last == "tokens"
}

func sensitiveToolFieldNameParts(key string) []string {
	runes := []rune(strings.TrimSpace(key))
	if len(runes) == 0 {
		return nil
	}
	var normalized strings.Builder
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			normalized.WriteRune('_')
			continue
		}
		if unicode.IsUpper(r) && i > 0 {
			previous := runes[i-1]
			var next rune
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			if unicode.IsLower(previous) || unicode.IsDigit(previous) ||
				(unicode.IsUpper(previous) && next != 0 && unicode.IsLower(next)) {
				normalized.WriteRune('_')
			}
		}
		normalized.WriteRune(unicode.ToLower(r))
	}
	return strings.FieldsFunc(normalized.String(), func(r rune) bool {
		return r == '_'
	})
}

func benignTokenMetric(parts []string) bool {
	if len(parts) != 2 {
		return false
	}
	switch parts[0] {
	case completionCommandKey, clientToolsPromptKey, listTotalField:
		return parts[1] == "tokens"
	default:
		return false
	}
}

func redactToolDiagnostic(value string) string {
	redactedClaims := taskpkg.RedactClaimTokens(strings.TrimSpace(value))
	claimMarker := "compozy_claim_" + "[REDACTED]"
	claimGuard := "__COMPOZY_REDACTED_CLAIM_" + "TOKEN__"
	protectedClaims := strings.ReplaceAll(redactedClaims, claimMarker, claimGuard)
	redacted := diagnostics.Redact(protectedClaims)
	return strings.ReplaceAll(redacted, claimGuard, claimMarker)
}
