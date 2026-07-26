package config

import (
	"fmt"

	"reflect"
	"sort"

	"strings"
	"time"
)

func configNodeFromValue(value reflect.Value, fieldName string) (any, bool) {
	value, ok := indirectConfigValue(value)
	if !ok {
		return nil, false
	}
	if value.Type() == configToolDurationType {
		return time.Duration(value.Int()).String(), true
	}
	switch value.Kind() {
	case reflect.Struct:
		return configStructNode(value)
	case reflect.Map:
		return configMapNode(value, fieldName)
	case reflect.Slice, reflect.Array:
		return configSequenceNode(value, fieldName)
	default:
		return configScalarNode(value)
	}
}

func indirectConfigValue(value reflect.Value) (reflect.Value, bool) {
	if !value.IsValid() {
		return reflect.Value{}, false
	}
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}, false
		}
		value = value.Elem()
	}
	return value, true
}

func configStructNode(value reflect.Value) (any, bool) {
	result := make(map[string]any)
	valueType := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := valueType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		fieldValue := value.Field(i)
		if field.Anonymous && field.Tag.Get("toml") == "" {
			node, hasValue := configNodeFromValue(fieldValue, "")
			if !hasValue {
				continue
			}
			embedded, isStruct := node.(map[string]any)
			if !isStruct {
				continue
			}
			for key, embeddedValue := range embedded {
				if _, exists := result[key]; !exists {
					result[key] = embeddedValue
				}
			}
			continue
		}
		name, omitEmpty, ok := tomlFieldName(field)
		if !ok {
			continue
		}
		if omitEmpty && fieldValue.IsZero() {
			continue
		}
		node, hasValue := configNodeFromValue(fieldValue, name)
		if hasValue {
			result[name] = node
		}
	}
	return result, true
}

func configMapNode(value reflect.Value, fieldName string) (any, bool) {
	if value.IsNil() {
		return map[string]any{}, true
	}
	result := make(map[string]any, value.Len())
	for _, key := range sortedReflectMapKeys(value) {
		mapKey := fmt.Sprint(key.Interface())
		if strings.EqualFold(fieldName, "env") || strings.EqualFold(fieldName, "secret_env") {
			result[mapKey] = RedactedValue()
			continue
		}
		node, hasValue := configNodeFromValue(value.MapIndex(key), "")
		if hasValue {
			result[mapKey] = node
		}
	}
	return result, true
}

func sortedReflectMapKeys(value reflect.Value) []reflect.Value {
	keys := value.MapKeys()
	sort.Slice(keys, func(i int, j int) bool {
		return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
	})
	return keys
}

func configSequenceNode(value reflect.Value, fieldName string) (any, bool) {
	items := make([]any, 0, value.Len())
	for i := 0; i < value.Len(); i++ {
		node, hasValue := configNodeFromValue(value.Index(i), fieldName)
		if hasValue {
			items = append(items, node)
		}
	}
	return items, true
}

func configScalarNode(value reflect.Value) (any, bool) {
	switch value.Kind() {
	case reflect.String:
		return value.String(), true
	case reflect.Bool:
		return value.Bool(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value.Uint(), true
	case reflect.Float32, reflect.Float64:
		return value.Float(), true
	default:
		if value.CanInterface() {
			return fmt.Sprint(value.Interface()), true
		}
		return nil, false
	}
}

func tomlFieldName(field reflect.StructField) (string, bool, bool) {
	tag := field.Tag.Get("toml")
	if tag == "-" {
		return "", false, false
	}
	if tag == "" {
		return strings.ToLower(field.Name), false, true
	}
	parts := strings.Split(tag, ",")
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return "", false, false
	}
	omitEmpty := false
	for _, part := range parts[1:] {
		if strings.TrimSpace(part) == "omitempty" {
			omitEmpty = true
			break
		}
	}
	return name, omitEmpty, true
}
