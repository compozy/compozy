package sqlite

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
)

// ToJSONText marshals a value to JSON (TEXT). Returns nil for nil inputs.
func ToJSONText(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return b, nil
}

// FromJSONText unmarshals JSON TEXT into a pointer receiver.
func FromJSONText[T any](src []byte, dst **T) error {
	if src == nil {
		*dst = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(src, &v); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}
	*dst = &v
	return nil
}

// JSONValue is a convenience wrapper implementing sql.Scanner and driver.Valuer.
// Use it in structs mapped to TEXT columns.
type JSONValue[T any] struct{ V *T }

func (j JSONValue[T]) Value() (driver.Value, error) { return ToJSONText(j.V) }

func (j *JSONValue[T]) Scan(src any) error {
	if src == nil {
		j.V = nil
		return nil
	}
	switch b := src.(type) {
	case []byte:
		return FromJSONText(b, &j.V)
	case string:
		return FromJSONText([]byte(b), &j.V)
	default:
		return fmt.Errorf("unsupported type for JSON scan: %T", src)
	}
}
