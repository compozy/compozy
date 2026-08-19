package loop

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

const (
	CodeInputTypeRequired        = "input_type_required"
	CodeInputTypeInvalid         = "input_type_invalid"
	CodeInputFieldUnknown        = "input_field_unknown"
	CodeInputRefRequired         = "input_ref_required"
	CodeInputRefKindInvalid      = "input_ref_kind_invalid"
	CodeInputEnumInvalid         = "input_enum_invalid"
	CodeInputDefaultInvalid      = "input_default_invalid"
	CodeRequestEntityKindInvalid = "request_entity_kind_invalid"
)

func (c *lintContext) lintInputs() {
	for _, name := range sortedInputKeys(c.def.Inputs) {
		input := c.def.Inputs[name]
		path := "inputs." + name
		c.lintInputExtras(path, input.Extra)
		if input.Type == "" {
			c.addPath(path+".type", CodeInputTypeRequired, "input %q type is required", name)
			continue
		}
		if !input.Type.Valid() {
			c.addPath(path+".type", CodeInputTypeInvalid, "input %q type %q is not supported", name, input.Type)
			continue
		}
		c.lintInputRef(name, path, input)
		c.lintInputEnum(name, path, input)
		if input.Default == nil {
			continue
		}
		if _, _, err := validateInputValue(name, input, input.Default); err != nil {
			c.addPath(path+".default", CodeInputDefaultInvalid, "%v", err)
		}
	}
}

func (c *lintContext) lintInputExtras(path string, extras map[string]any) {
	keys := make([]string, 0, len(extras))
	for key := range extras {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		c.addPath(path+"."+key, CodeInputFieldUnknown, "input field %q is unknown", key)
	}
}

func (c *lintContext) lintInputRef(name string, path string, input dsl.Input) {
	if input.Type != dsl.InputTypeRef {
		if input.Ref != nil {
			c.addPath(path+".ref", CodeInputFieldUnknown, "input %q declares ref for type %q", name, input.Type)
		}
		return
	}
	if input.Ref == nil {
		c.addPath(path+".ref", CodeInputRefRequired, "input %q requires ref.kind", name)
		return
	}
	c.lintInputExtras(path+".ref", input.Ref.Extra)
	if !input.Ref.Kind.Valid() {
		c.addPath(
			path+".ref.kind",
			CodeInputRefKindInvalid,
			"input %q ref kind %q is not supported",
			name,
			input.Ref.Kind,
		)
	}
}

func (c *lintContext) lintInputEnum(name string, path string, input dsl.Input) {
	if input.Enum == nil {
		return
	}
	if input.Type != dsl.InputTypeString {
		c.addPath(path+".enum", CodeInputEnumInvalid, "input %q enum requires type string", name)
		return
	}
	if len(input.Enum) == 0 {
		c.addPath(path+".enum", CodeInputEnumInvalid, "input %q enum must not be empty", name)
		return
	}
	seen := make([]string, 0, len(input.Enum))
	for index, value := range input.Enum {
		if strings.TrimSpace(value) == "" {
			c.addPath(
				fmt.Sprintf("%s.enum.%d", path, index),
				CodeInputEnumInvalid,
				"input %q enum values must not be blank",
				name,
			)
			continue
		}
		if slices.Contains(seen, value) {
			c.addPath(
				fmt.Sprintf("%s.enum.%d", path, index),
				CodeInputEnumInvalid,
				"input %q enum value %q is duplicated",
				name,
				value,
			)
			continue
		}
		seen = append(seen, value)
	}
}

func (c *lintContext) addPath(path string, code string, format string, args ...any) {
	c.errors = append(c.errors, LintError{
		Path: strings.TrimSpace(path), Code: code,
		Message: fmt.Sprintf(format, args...), Severity: SeverityError,
	})
}
