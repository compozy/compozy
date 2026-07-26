package aghsdk

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

const (
	digestErrorKey = "error"
)

// CanonicalJSON returns RFC 8785/JCS-compatible canonical JSON bytes.
func CanonicalJSON(raw json.RawMessage) ([]byte, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, NewInvalidParamsError("json value is required", nil)
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, NewInvalidParamsError(err.Error(), nil)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return nil, NewInvalidParamsError(err.Error(), nil)
		}
		return nil, NewInvalidParamsError("multiple JSON values are not allowed", nil)
	}

	var builder strings.Builder
	if err := appendCanonicalValue(&builder, value); err != nil {
		return nil, err
	}
	return []byte(builder.String()), nil
}

// SchemaDigest returns the lowercase SHA-256 digest of a canonical JSON Schema subtree.
func SchemaDigest(raw json.RawMessage) (string, error) {
	if err := validateJSONObject("schema", raw, true); err != nil {
		return "", err
	}
	canonical, err := CanonicalJSON(raw)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
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
		return NewInvalidParamsError(fmt.Sprintf("unsupported JSON value %T", value), nil)
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
		return "", NewInvalidParamsError(err.Error(), nil)
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

func normalizeSchema(value any, field string, required bool) (json.RawMessage, error) {
	if value == nil {
		if required {
			return nil, NewInvalidParamsError(field+" is required", nil)
		}
		return nil, nil
	}
	raw, err := marshalRawJSON(value)
	if err != nil {
		return nil, NewInvalidParamsError(
			field+" must be JSON serializable",
			map[string]any{digestErrorKey: err.Error()},
		)
	}
	if err := validateJSONObject(field, raw, required); err != nil {
		return nil, err
	}
	return raw, nil
}

func validateJSONObject(field string, raw json.RawMessage, required bool) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		if required {
			return NewInvalidParamsError(field+" object is required", nil)
		}
		return nil
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return NewInvalidParamsError(field+" must be a JSON object", map[string]any{digestErrorKey: err.Error()})
	}
	if decoded == nil && required {
		return NewInvalidParamsError(field+" must be a JSON object", nil)
	}
	return nil
}

func marshalRawJSON(value any) (json.RawMessage, error) {
	switch typed := value.(type) {
	case json.RawMessage:
		return cloneRawMessage(typed), nil
	case []byte:
		return cloneRawMessage(typed), nil
	case string:
		return json.RawMessage(strings.TrimSpace(typed)), nil
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return encoded, nil
	}
}

func cloneRawMessage(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	cloned := make(json.RawMessage, len(src))
	copy(cloned, src)
	return cloned
}
