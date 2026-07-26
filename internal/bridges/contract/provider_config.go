package contract

import (
	"encoding/json"
	"fmt"
	"sort"
)

var operatorOwnedProviderDestinationFields = map[string]struct{}{
	"api_base_url": {}, "oauth_token_url": {}, "openid_metadata_url": {},
	"service_url": {}, "token_url": {},
}

// NormalizeProviderConfigJSON validates and canonicalizes provider-owned
// configuration while rejecting operator-owned destinations.
func NormalizeProviderConfigJSON(value json.RawMessage) (json.RawMessage, error) {
	return normalizeProviderConfigJSON(value)
}

func normalizeProviderConfigJSON(value json.RawMessage) (json.RawMessage, error) {
	normalized, err := normalizeJSONObject(value, "bridge instance provider config")
	if err != nil || len(normalized) == 0 {
		return normalized, err
	}
	var config map[string]any
	if err := json.Unmarshal(normalized, &config); err != nil {
		return nil, fmt.Errorf("bridges: decode bridge instance provider config: %w", err)
	}
	if err := rejectOperatorOwnedProviderDestinations(config, "provider_config"); err != nil {
		return nil, err
	}
	return normalized, nil
}

func rejectOperatorOwnedProviderDestinations(value any, path string) error {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fieldPath := path + "." + key
			if _, blocked := operatorOwnedProviderDestinationFields[key]; blocked {
				return fmt.Errorf(
					"bridges: %s is operator-owned and must be configured through the adapter process environment",
					fieldPath,
				)
			}
			if err := rejectOperatorOwnedProviderDestinations(typed[key], fieldPath); err != nil {
				return err
			}
		}
	case []any:
		for index, item := range typed {
			if err := rejectOperatorOwnedProviderDestinations(item, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	}
	return nil
}
