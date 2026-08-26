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
	if path, found := findSecretField(decoded, "$"); found {
		return nil, nil, newError(
			CodeRedactionConflict,
			FaultChild,
			fmt.Sprintf("result contains secret-shaped field %s; return a *_hash field or a non-secret reference instead", path),
			nil,
		)
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

func findSecretField(value any, path string) (string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			childPath := appendPath(path, key)
			if deniedSecretKey(key) {
				return childPath, true
			}
			if foundPath, found := findSecretField(child, childPath); found {
				return foundPath, true
			}
		}
	case []any:
		for index, child := range typed {
			if foundPath, found := findSecretField(child, fmt.Sprintf("%s[%d]", path, index)); found {
				return foundPath, true
			}
		}
	}
	return "", false
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
	return walkSecretFieldAuthorship(schema, schema, "$", make(map[string]struct{}))
}

func walkSecretFieldAuthorship(root, schema map[string]any, path string, visiting map[string]struct{}) error {
	if reference, ok := schema["$ref"].(string); ok && strings.HasPrefix(reference, "#/") {
		if _, cycle := visiting[reference]; !cycle {
			resolved, found := resolveLocalSchemaRef(root, reference)
			if found {
				visiting[reference] = struct{}{}
				if err := walkSecretFieldAuthorship(root, resolved, path, visiting); err != nil {
					return err
				}
				delete(visiting, reference)
			}
		}
	}
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
				if err := walkSecretFieldAuthorship(root, childSchema, appendPath(path, name), visiting); err != nil {
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
			if err := walkSecretFieldAuthorship(root, child, path, visiting); err != nil {
				return err
			}
		}
	}
	for _, keyword := range []string{schemaAllOf, schemaAnyOf, schemaOneOf, schemaPrefixItems} {
		if children, ok := schema[keyword].([]any); ok {
			for _, child := range children {
				if childSchema, ok := child.(map[string]any); ok {
					if err := walkSecretFieldAuthorship(root, childSchema, path, visiting); err != nil {
						return err
					}
				}
			}
		}
	}
	for _, keyword := range []string{"$defs", schemaDependentSchemas} {
		if definitions, ok := schema[keyword].(map[string]any); ok {
			for name, child := range definitions {
				if childSchema, ok := child.(map[string]any); ok {
					if err := walkSecretFieldAuthorship(root, childSchema, appendPath(path, name), visiting); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func resolveLocalSchemaRef(root map[string]any, reference string) (map[string]any, bool) {
	var current any = root
	for _, token := range strings.Split(strings.TrimPrefix(reference, "#/"), "/") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		current, ok = object[token]
		if !ok {
			return nil, false
		}
	}
	resolved, ok := current.(map[string]any)
	return resolved, ok
}
