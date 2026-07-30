package extensionpkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"slices"
	"strings"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const commandSchemaResourceURL = "https://compozy.invalid/extension-command-input.json"

const commandSchemaStringType = "string"

func projectCommandFlags(
	rawSchema json.RawMessage,
	mappings map[string]string,
	fieldPath string,
) ([]extensioncontract.CommandFlag, error) {
	if len(mappings) == 0 {
		return nil, nil
	}
	root, err := compileCommandSchema(rawSchema)
	if err != nil {
		fields := make([]string, 0, len(mappings))
		for _, field := range mappings {
			fields = append(fields, strings.TrimSpace(field))
		}
		slices.Sort(fields)
		fields = slices.Compact(fields)
		return nil, &ManifestValidationError{
			Field: fieldPath,
			Message: fmt.Sprintf(
				"projected fields %s cannot be resolved from the input schema: %v; use --input for these fields",
				strings.Join(fields, ", "),
				err,
			),
		}
	}
	root, err = commandSchemaTarget(root)
	if err != nil {
		return nil, &ManifestValidationError{Field: fieldPath, Message: err.Error()}
	}
	if err := rejectCommandSchemaComposition(root); err != nil {
		return nil, &ManifestValidationError{
			Field: fieldPath, Message: err.Error() + "; use --input for unsupported fields",
		}
	}
	if !commandSchemaHasType(root, "object") || len(root.Properties) == 0 {
		return nil, &ManifestValidationError{
			Field: fieldPath,
			Message: "projected flags require a top-level object input schema; " +
				"use --input for unsupported fields",
		}
	}
	required := make(map[string]struct{}, len(root.Required))
	for _, field := range root.Required {
		required[field] = struct{}{}
	}

	flags := make([]extensioncontract.CommandFlag, 0, len(mappings))
	for name, mappedField := range mappings {
		flagName := strings.TrimSpace(name)
		field := strings.TrimSpace(mappedField)
		if err := validateProjectedFlagName(fieldPath, flagName); err != nil {
			return nil, err
		}
		property, ok := root.Properties[field]
		if !ok {
			return nil, &ManifestValidationError{
				Field: fieldPath + "." + flagName, Value: field,
				Message: fmt.Sprintf("flag %q references missing top-level input field %q", flagName, field),
			}
		}
		flag, err := projectCommandFlag(flagName, field, property)
		if err != nil {
			return nil, &ManifestValidationError{
				Field: fieldPath + "." + flagName, Value: field,
				Message: fmt.Sprintf("flag %q field %q: %v; use --input for this field", flagName, field, err),
			}
		}
		_, flag.Required = required[field]
		flags = append(flags, flag)
	}
	slices.SortFunc(flags, func(left, right extensioncontract.CommandFlag) int {
		return strings.Compare(left.Name, right.Name)
	})
	return flags, nil
}

func validateProjectedFlagName(fieldPath, name string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsAny(name, "=/") {
		return &ManifestValidationError{Field: fieldPath, Value: name, Message: "command flag name is invalid"}
	}
	if _, reserved := reservedCommandFlags[name]; reserved {
		return &ManifestValidationError{
			Field: fieldPath + "." + name, Value: name,
			Message: "command flag is reserved; reserved flags: " + reservedCommandFlagList,
		}
	}
	return nil
}

func compileCommandSchema(raw json.RawMessage) (*jsonschema.Schema, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("schema is required")
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	if err := compiler.AddResource(commandSchemaResourceURL, document); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}
	compiled, err := compiler.Compile(commandSchemaResourceURL)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	return compiled, nil
}

func projectCommandFlag(name, field string, schema *jsonschema.Schema) (extensioncontract.CommandFlag, error) {
	target, err := commandSchemaTarget(schema)
	if err != nil {
		return extensioncontract.CommandFlag{}, err
	}
	if err := rejectCommandSchemaComposition(target); err != nil {
		return extensioncontract.CommandFlag{}, err
	}
	flag := extensioncontract.CommandFlag{Name: name, Field: field}
	types := commandSchemaTypes(target)
	if slices.Equal(types, []string{"array"}) {
		item, err := commandArrayItem(target)
		if err != nil {
			return extensioncontract.CommandFlag{}, err
		}
		item, err = commandSchemaTarget(item)
		if err != nil {
			return extensioncontract.CommandFlag{}, err
		}
		if err := rejectCommandSchemaComposition(item); err != nil {
			return extensioncontract.CommandFlag{}, err
		}
		flag.Type, flag.Nullable, err = commandScalarType(item)
		if err != nil {
			return extensioncontract.CommandFlag{}, fmt.Errorf("array items: %w", err)
		}
		if flag.Nullable {
			return extensioncontract.CommandFlag{}, fmt.Errorf("array items must be one non-null scalar")
		}
		flag.Repeatable = true
		if err := populateCommandFlagConstraints(&flag, item, target); err != nil {
			return extensioncontract.CommandFlag{}, err
		}
		return flag, nil
	}
	flag.Type, flag.Nullable, err = commandScalarType(target)
	if err != nil {
		return extensioncontract.CommandFlag{}, err
	}
	if err := populateCommandFlagConstraints(&flag, target, target); err != nil {
		return extensioncontract.CommandFlag{}, err
	}
	return flag, nil
}

func commandSchemaTarget(schema *jsonschema.Schema) (*jsonschema.Schema, error) {
	seen := make(map[*jsonschema.Schema]struct{})
	for schema != nil && schema.Ref != nil {
		if _, exists := seen[schema]; exists {
			return nil, fmt.Errorf("cyclic $ref is not projectable")
		}
		seen[schema] = struct{}{}
		schema = schema.Ref
	}
	if schema == nil {
		return nil, fmt.Errorf("unresolvable $ref")
	}
	return schema, nil
}

func rejectCommandSchemaComposition(schema *jsonschema.Schema) error {
	if len(schema.AllOf) > 0 || len(schema.AnyOf) > 0 || len(schema.OneOf) > 0 || schema.Not != nil ||
		schema.If != nil || schema.Then != nil || schema.Else != nil ||
		schema.DynamicRef != nil || schema.RecursiveRef != nil {
		return fmt.Errorf("composed schemas are not projectable")
	}
	return nil
}

func commandSchemaTypes(schema *jsonschema.Schema) []string {
	if schema == nil || schema.Types == nil {
		return nil
	}
	return schema.Types.ToStrings()
}

func commandSchemaHasType(schema *jsonschema.Schema, want string) bool {
	return slices.Equal(commandSchemaTypes(schema), []string{want})
}

func commandScalarType(schema *jsonschema.Schema) (extensioncontract.CommandFlagType, bool, error) {
	types := commandSchemaTypes(schema)
	nullable := false
	if len(types) == 2 && slices.Contains(types, manifestNullKey) {
		nullable = true
		types = slices.DeleteFunc(
			slices.Clone(types),
			func(value string) bool { return value == manifestNullKey },
		)
	}
	if len(types) != 1 {
		return "", false, fmt.Errorf("type must be one scalar or a nullable scalar")
	}
	switch types[0] {
	case commandSchemaStringType:
		return extensioncontract.CommandFlagString, nullable, nil
	case "boolean":
		return extensioncontract.CommandFlagBoolean, nullable, nil
	case "integer":
		return extensioncontract.CommandFlagInteger, nullable, nil
	case "number":
		return extensioncontract.CommandFlagNumber, nullable, nil
	default:
		return "", false, fmt.Errorf("type %q is not a supported scalar", types[0])
	}
}

func commandArrayItem(schema *jsonschema.Schema) (*jsonschema.Schema, error) {
	if len(schema.PrefixItems) > 0 {
		return nil, fmt.Errorf("tuple arrays are not projectable")
	}
	if schema.Items2020 != nil {
		return schema.Items2020, nil
	}
	switch items := schema.Items.(type) {
	case *jsonschema.Schema:
		return items, nil
	case []*jsonschema.Schema:
		return nil, fmt.Errorf("tuple arrays are not projectable")
	default:
		return nil, fmt.Errorf("array must declare exactly one scalar items schema")
	}
}

func populateCommandFlagConstraints(
	flag *extensioncontract.CommandFlag,
	constraintSchema *jsonschema.Schema,
	defaultSchema *jsonschema.Schema,
) error {
	enum, err := commandEnumValues(constraintSchema, flag.Type)
	if err != nil {
		return err
	}
	flag.Enum = enum
	if defaultSchema.Default != nil {
		flag.Default, err = canonicalCommandJSON(*defaultSchema.Default)
		if err != nil {
			return fmt.Errorf("default: %w", err)
		}
	}
	flag.Minimum, err = commandBound(constraintSchema.Minimum)
	if err != nil {
		return fmt.Errorf("minimum: %w", err)
	}
	flag.Maximum, err = commandBound(constraintSchema.Maximum)
	if err != nil {
		return fmt.Errorf("maximum: %w", err)
	}
	return nil
}

func commandEnumValues(schema *jsonschema.Schema, flagType extensioncontract.CommandFlagType) ([]string, error) {
	if schema.Enum == nil {
		return nil, nil
	}
	values := make([]string, 0, len(schema.Enum.Values))
	for _, value := range schema.Enum.Values {
		if !commandEnumMatchesType(value, flagType) {
			return nil, fmt.Errorf("enum value does not match projected type %q", flagType)
		}
		if text, ok := value.(string); ok {
			values = append(values, text)
			continue
		}
		raw, err := canonicalCommandJSON(value)
		if err != nil {
			return nil, fmt.Errorf("enum: %w", err)
		}
		values = append(values, string(raw))
	}
	return values, nil
}

func commandEnumMatchesType(value any, flagType extensioncontract.CommandFlagType) bool {
	switch flagType {
	case extensioncontract.CommandFlagString:
		_, ok := value.(string)
		return ok
	case extensioncontract.CommandFlagBoolean:
		_, ok := value.(bool)
		return ok
	case extensioncontract.CommandFlagInteger:
		number, ok := value.(json.Number)
		if !ok {
			return false
		}
		rat, ok := new(big.Rat).SetString(number.String())
		return ok && rat.IsInt()
	case extensioncontract.CommandFlagNumber:
		_, ok := value.(json.Number)
		return ok
	default:
		return false
	}
}

func canonicalCommandJSON(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return toolspkg.CanonicalJSON(raw)
}

func commandBound(value *big.Rat) (*float64, error) {
	if value == nil {
		return nil, nil
	}
	converted, _ := value.Float64()
	if math.IsInf(converted, 0) || math.IsNaN(converted) {
		return nil, fmt.Errorf("bound is outside float64 range")
	}
	return &converted, nil
}
