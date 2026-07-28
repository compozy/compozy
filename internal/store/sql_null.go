package store

import (
	"database/sql"
	"strings"
)

// SQLNullString maps an optional string to a canonical trimmed sql.NullString.
func SQLNullString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	return sql.NullString{String: trimmed, Valid: trimmed != ""}
}

// SQLNullStringPointer maps a nil or blank string pointer to an invalid sql.NullString.
func SQLNullStringPointer(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return SQLNullString(*value)
}

// NullableString maps blank strings to SQL NULL.
func NullableString(value string) any {
	nullable := SQLNullString(value)
	if !nullable.Valid {
		return nil
	}
	return nullable.String
}

// NullableStringPointer maps nil or blank string pointers to SQL NULL.
func NullableStringPointer(value *string) any {
	nullable := SQLNullStringPointer(value)
	if !nullable.Valid {
		return nil
	}
	return nullable.String
}

// NullableInt64 maps nil pointers to SQL NULL.
func NullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

// NullableFloat64 maps nil pointers to SQL NULL.
func NullableFloat64(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

// NullString converts sql.NullString into a trimmed string pointer.
func NullString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// NullInt64 converts sql.NullInt64 into a pointer.
func NullInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}

// NullFloat64 converts sql.NullFloat64 into a pointer.
func NullFloat64(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	v := value.Float64
	return &v
}
