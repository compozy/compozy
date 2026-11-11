package refs

// IsFullJSONSchema distinguishes schema keywords from shorthand field maps.
func IsFullJSONSchema(schema map[string]any) bool {
	fullKeys := map[string]struct{}{
		"$defs":                 {},
		"$id":                   {},
		"$schema":               {},
		"additionalProperties":  {},
		"allOf":                 {},
		"anyOf":                 {},
		"contains":              {},
		"contentEncoding":       {},
		"contentMediaType":      {},
		"const":                 {},
		"default":               {},
		"dependentRequired":     {},
		"dependentSchemas":      {},
		"description":           {},
		"enum":                  {},
		"examples":              {},
		"exclusiveMaximum":      {},
		"exclusiveMinimum":      {},
		"format":                {},
		"if":                    {},
		"items":                 {},
		"maxItems":              {},
		"maxLength":             {},
		"maxProperties":         {},
		"maximum":               {},
		"minItems":              {},
		"minLength":             {},
		"minProperties":         {},
		"minimum":               {},
		"multipleOf":            {},
		"not":                   {},
		"oneOf":                 {},
		"pattern":               {},
		"prefixItems":           {},
		"properties":            {},
		"propertyNames":         {},
		"required":              {},
		"then":                  {},
		"title":                 {},
		"type":                  {},
		"unevaluatedProperties": {},
		"x-compozy-kind":        {},
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
