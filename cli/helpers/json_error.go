package helpers

import "errors"

// jsonHandledError marks errors already rendered as JSON to stdout.
type jsonHandledError struct {
	message string
}

func (e *jsonHandledError) Error() string {
	return e.message
}

// NewJSONHandledError returns an error signaling that the JSON response has been emitted.
func NewJSONHandledError(message string) error {
	return &jsonHandledError{message: message}
}

// IsJSONHandledError reports whether err represents an already rendered JSON response.
func IsJSONHandledError(err error) bool {
	var target *jsonHandledError
	return errors.As(err, &target)
}
