package schedule

import (
	"errors"
	"fmt"
	"strings"
)

// Common errors
var (
	ErrScheduleNotFound = errors.New("schedule not found")
	ErrInvalidSchedule  = errors.New("invalid schedule configuration")
)

// MultiError aggregates multiple errors into a single error
type MultiError struct {
	Errors []error
}

func (m *MultiError) Error() string {
	if len(m.Errors) == 0 {
		return ""
	}
	if len(m.Errors) == 1 {
		return m.Errors[0].Error()
	}
	var msgs []string
	for i, err := range m.Errors {
		msgs = append(msgs, fmt.Sprintf("[%d] %v", i+1, err))
	}
	return fmt.Sprintf("multiple errors occurred:\n%s", strings.Join(msgs, "\n"))
}

// AppendError adds an error to the MultiError
func AppendError(multi *MultiError, err error) *MultiError {
	if err == nil {
		return multi
	}
	if multi == nil {
		multi = &MultiError{}
	}
	multi.Errors = append(multi.Errors, err)
	return multi
}
