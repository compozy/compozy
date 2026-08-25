package contracts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var deniedSecretKeys = map[string]struct{}{
	"apikey": {}, "accesstoken": {}, "claimtoken": {}, "clientsecret": {},
	"password": {}, "privatekey": {}, "refreshtoken": {}, "secret": {}, "token": {},
}

// RedactPreservingContract sanitizes values and proves the transformed payload still validates.
func RedactPreservingContract(contract Contract, payload json.RawMessage) (json.RawMessage, []Redaction, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, nil, fmt.Errorf("decode result for redaction: %w", err)
	}
	redactions := make([]Redaction, 0, 2)
	redacted := redactValue(decoded, "$", &redactions)
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return nil, nil, fmt.Errorf("encode redacted result: %w", err)
	}
	compiled, err := compileSchema(contract)
	if err != nil {
		return nil, nil, newError(CodeContractCompile, FaultContract, err.Error(), err)
	}
	verdict := validatePayload(compiled, encoded)
	if !verdict.Valid {
		cause := errors.New(BuildRepairPrompt(verdict.Issues))
		return nil, redactions, newError(
			CodeRedactionConflict,
			FaultChild,
			"result contains secret material in a contract-constrained field; remove it and return a safe value",
			cause,
		)
	}
	sort.Slice(redactions, func(i, j int) bool { return redactions[i].Path < redactions[j].Path })
	return encoded, redactions, nil
}

func redactValue(value any, path string, redactions *[]Redaction) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			childPath := appendPath(path, key)
			if deniedSecretKey(key) {
				fingerprint := secretFingerprint(fmt.Sprint(child))
				result[key] = redactionMarker(fingerprint)
				*redactions = append(*redactions, Redaction{Path: childPath, Fingerprint: fingerprint})
				continue
			}
			result[key] = redactValue(child, childPath, redactions)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = redactValue(child, fmt.Sprintf("%s[%d]", path, index), redactions)
		}
		return result
	case string:
		clean, found, reject := SanitizeText(typed)
		if reject && len(found) > 0 {
			clean = redactionMarker(found[0].Fingerprint)
		}
		for _, item := range found {
			item.Path = path
			*redactions = append(*redactions, item)
		}
		return clean
	default:
		return value
	}
}

func deniedSecretKey(key string) bool {
	normalized := normalizeFieldName(key)
	if strings.HasSuffix(normalized, "hash") {
		return false
	}
	_, denied := deniedSecretKeys[normalized]
	return denied
}

func normalizeFieldName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("_", "", "-", "", ".", "").Replace(value)
	return value
}

func appendPath(path, key string) string {
	if path == "$" {
		return path + "." + key
	}
	return path + "." + key
}

func validateSecretFieldAuthorship(canonical json.RawMessage) error {
	var schema map[string]any
	if err := json.Unmarshal(canonical, &schema); err != nil {
		return fmt.Errorf("inspect schema secret fields: %w", err)
	}
	return walkSecretFieldAuthorship(schema, "$")
}

func walkSecretFieldAuthorship(schema map[string]any, path string) error {
	required := make(map[string]struct{})
	if values, ok := schema[schemaRequired].([]any); ok {
		for _, value := range values {
			if name, ok := value.(string); ok {
				required[name] = struct{}{}
			}
		}
	}
	if properties, ok := schema[schemaProperties].(map[string]any); ok {
		for name, child := range properties {
			if _, isRequired := required[name]; isRequired && deniedSecretKey(name) {
				return fmt.Errorf(
					"required field %s.%s is secret-shaped; return a *_hash field or a non-secret reference instead",
					path,
					name,
				)
			}
			if childSchema, ok := child.(map[string]any); ok {
				if err := walkSecretFieldAuthorship(childSchema, appendPath(path, name)); err != nil {
					return err
				}
			}
		}
	}
	for _, keyword := range []string{
		schemaItems,
		schemaContains,
		schemaIf,
		schemaThen,
		schemaElse,
		schemaAdditionalProperties,
		schemaUnevaluatedProperties,
	} {
		if child, ok := schema[keyword].(map[string]any); ok {
			if err := walkSecretFieldAuthorship(child, path); err != nil {
				return err
			}
		}
	}
	for _, keyword := range []string{schemaAllOf, schemaAnyOf, schemaOneOf, schemaPrefixItems} {
		if children, ok := schema[keyword].([]any); ok {
			for _, child := range children {
				if childSchema, ok := child.(map[string]any); ok {
					if err := walkSecretFieldAuthorship(childSchema, path); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
