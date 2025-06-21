package store

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
)

// ToJSONB marshals a value to JSONB-compatible bytes, returning nil for nil input.
func ToJSONB(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}

	// Handle typed nil pointers (e.g., (*core.Output)(nil))
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return nil, nil
	}

	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshaling to jsonb: %w", err)
	}
	return data, nil
}

// FromJSONB unmarshals JSONB data into a pointer, setting nil if the source is nil.
func FromJSONB[T any](src []byte, dst **T) error {
	if src == nil {
		*dst = nil
		return nil
	}
	var target T
	if err := json.Unmarshal(src, &target); err != nil {
		return fmt.Errorf("unmarshaling from jsonb: %w", err)
	}
	*dst = &target
	return nil
}

// ScanRows scans rows into a destination using pgxscan
func ScanRows(_ context.Context, rows pgx.Rows, dst any) error {
	return pgxscan.ScanAll(dst, rows)
}
