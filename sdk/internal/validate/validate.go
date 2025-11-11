// Package validate provides helper functions for validating SDK builder inputs.
package validate

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

func ensureContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	return nil
}

func ensureFieldName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("field name is required")
	}
	return nil
}

func isEmptyCollection(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice:
		return v.Len() == 0
	}
	return false
}

func isStringValue(v reflect.Value) (string, bool) {
	if v.Kind() == reflect.String {
		return v.String(), true
	}
	return "", false
}

// Required checks that a required field is present and not empty.
func Required(ctx context.Context, name string, value any) error {
	if err := ensureContext(ctx); err != nil {
		return err
	}
	if err := ensureFieldName(name); err != nil {
		return err
	}
	if value == nil {
		return fmt.Errorf("%s is required", name)
	}

	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return fmt.Errorf("%s is required", name)
		}
		rv = rv.Elem()
	}

	if !rv.IsValid() {
		return fmt.Errorf("%s is required", name)
	}

	if isEmptyCollection(rv) {
		return fmt.Errorf("%s cannot be empty", name)
	}

	if str, ok := isStringValue(rv); ok {
		if strings.TrimSpace(str) == "" {
			return fmt.Errorf("%s cannot be empty", name)
		}
	}

	return nil
}

// ID ensures that an identifier contains only alphanumeric characters and hyphens.
func ID(ctx context.Context, id string) error {
	if err := ensureContext(ctx); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fmt.Errorf("id is required")
	}
	if !idPattern.MatchString(trimmed) {
		return fmt.Errorf("id must contain only letters, numbers, or hyphens")
	}
	return nil
}

// NonEmpty ensures a string field is not empty or whitespace.
func NonEmpty(ctx context.Context, name, value string) error {
	if err := ensureContext(ctx); err != nil {
		return err
	}
	if err := ensureFieldName(name); err != nil {
		return err
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s cannot be empty", name)
	}
	return nil
}

// URL ensures the provided string is a well-formed URL with scheme and host.
func URL(ctx context.Context, rawURL string) error {
	if err := ensureContext(ctx); err != nil {
		return err
	}
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return fmt.Errorf("url is required")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("url must be valid: %w", err)
	}
	if parsed.Scheme == "" {
		return fmt.Errorf("url must include a scheme such as http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("url must include a host")
	}
	return nil
}

// Duration ensures a duration is strictly positive.
func Duration(ctx context.Context, d time.Duration) error {
	if err := ensureContext(ctx); err != nil {
		return err
	}
	if d <= 0 {
		return fmt.Errorf("duration must be positive: got %s", d)
	}
	return nil
}

// Range ensures an integer value lies within the inclusive range.
func Range(ctx context.Context, name string, val, minVal, maxVal int) error {
	if err := ensureContext(ctx); err != nil {
		return err
	}
	if err := ensureFieldName(name); err != nil {
		return err
	}
	if minVal > maxVal {
		return fmt.Errorf("%s range is invalid: min %d is greater than max %d", name, minVal, maxVal)
	}
	if val < minVal || val > maxVal {
		return fmt.Errorf("%s must be between %d and %d inclusive: got %d", name, minVal, maxVal, val)
	}
	return nil
}

// Cron ensures the provided cron expression is valid according to cron/v3 standard parsing rules.
func Cron(ctx context.Context, expr string) error {
	if err := ensureContext(ctx); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return fmt.Errorf("cron expression is required")
	}
	if _, err := cron.ParseStandard(trimmed); err != nil {
		return fmt.Errorf("cron expression is invalid: %w", err)
	}
	return nil
}
