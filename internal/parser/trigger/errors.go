package trigger

import (
	"errors"
	"fmt"
)

// Common sentinel errors
var (
	ErrInvalidType        = errors.New("invalid trigger type")
	ErrMissingWebhook     = errors.New("webhook configuration is required for webhook trigger type")
	ErrInvalidInputSchema = errors.New("invalid input schema")
)

// Error constructors
func NewInvalidTypeError(triggerType string) error {
	return fmt.Errorf("%w: %s", ErrInvalidType, triggerType)
}

func NewMissingWebhookError() error {
	return ErrMissingWebhook
}

func NewInvalidInputSchemaError(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidInputSchema, err)
}
