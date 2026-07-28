package tools

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// CanonicalJSON returns RFC 8785/JCS-compatible canonical JSON bytes.
func CanonicalJSON(raw json.RawMessage) ([]byte, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, NewValidationError("json", ReasonSchemaInvalid, "json value is required")
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, NewValidationError("json", ReasonSchemaInvalid, err.Error())
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return nil, NewValidationError("json", ReasonSchemaInvalid, err.Error())
		}
		return nil, NewValidationError("json", ReasonSchemaInvalid, "multiple JSON values are not allowed")
	}

	var builder strings.Builder
	if err := appendCanonicalValue(&builder, value); err != nil {
		return nil, err
	}
	return []byte(builder.String()), nil
}

// SchemaDigest returns the lowercase SHA-256 digest of a canonical JSON Schema subtree.
func SchemaDigest(raw json.RawMessage) (string, error) {
	if err := ValidateJSONObject("schema", raw, true); err != nil {
		return "", err
	}
	canonical, err := CanonicalJSON(raw)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// DescriptorWithSchemaDigests returns a descriptor with canonical schema digests populated.
func DescriptorWithSchemaDigests(descriptor Descriptor) (Descriptor, error) {
	descriptor.ToolPresentation = CloneToolPresentation(descriptor.ToolPresentation)
	inputDigest, err := SchemaDigest(descriptor.InputSchema)
	if err != nil {
		return Descriptor{}, wrapField(err, "input_schema")
	}
	if existing := strings.TrimSpace(descriptor.InputSchemaDigest); existing != "" && existing != inputDigest {
		return Descriptor{}, NewValidationError(
			"input_schema_digest",
			ReasonRuntimeDescriptorMismatch,
			"input schema digest does not match input_schema",
		)
	}
	descriptor.InputSchemaDigest = inputDigest

	if len(bytes.TrimSpace(descriptor.OutputSchema)) == 0 {
		descriptor.OutputSchemaDigest = ""
		return descriptor, nil
	}
	outputDigest, err := SchemaDigest(descriptor.OutputSchema)
	if err != nil {
		return Descriptor{}, wrapField(err, "output_schema")
	}
	if existing := strings.TrimSpace(descriptor.OutputSchemaDigest); existing != "" && existing != outputDigest {
		return Descriptor{}, NewValidationError(
			"output_schema_digest",
			ReasonRuntimeDescriptorMismatch,
			"output schema digest does not match output_schema",
		)
	}
	descriptor.OutputSchemaDigest = outputDigest
	return descriptor, nil
}

func appendCanonicalValue(builder *strings.Builder, value any) error {
	switch typed := value.(type) {
	case nil:
		builder.WriteString("null")
	case bool:
		if typed {
			builder.WriteString("true")
		} else {
			builder.WriteString("false")
		}
	case string:
		builder.WriteString(quoteJSONString(typed))
	case json.Number:
		number, err := canonicalNumber(typed)
		if err != nil {
			return err
		}
		builder.WriteString(number)
	case []any:
		builder.WriteByte('[')
		for i, item := range typed {
			if i > 0 {
				builder.WriteByte(',')
			}
			if err := appendCanonicalValue(builder, item); err != nil {
				return err
			}
		}
		builder.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		builder.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				builder.WriteByte(',')
			}
			builder.WriteString(quoteJSONString(key))
			builder.WriteByte(':')
			if err := appendCanonicalValue(builder, typed[key]); err != nil {
				return err
			}
		}
		builder.WriteByte('}')
	default:
		return NewValidationError("json", ReasonSchemaInvalid, fmt.Sprintf("unsupported JSON value %T", value))
	}
	return nil
}

func quoteJSONString(value string) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return strconv.Quote(value)
	}
	return strings.TrimSpace(buffer.String())
}

func canonicalNumber(number json.Number) (string, error) {
	raw := number.String()
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return "", NewValidationError("json.number", ReasonSchemaInvalid, err.Error())
	}
	if parsed == 0 {
		return "0", nil
	}
	if !strings.ContainsAny(raw, ".eE") {
		return raw, nil
	}
	canonical := strconv.FormatFloat(parsed, 'g', -1, 64)
	canonical = strings.ReplaceAll(canonical, "e+", "e")
	return canonical, nil
}
