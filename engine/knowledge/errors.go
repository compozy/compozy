package knowledge

import "strings"

type ValidationErrors struct {
	errors []error
}

// NewValidationErrors builds a ValidationErrors value from the provided list, ignoring nil entries.
func NewValidationErrors(errs ...error) ValidationErrors {
	out := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			out = append(out, err)
		}
	}
	return ValidationErrors{errors: out}
}

// Add appends a non-nil validation error to the collection.
func (v *ValidationErrors) Add(err error) {
	if v == nil || err == nil {
		return
	}
	v.errors = append(v.errors, err)
}

func (v ValidationErrors) Error() string {
	if len(v.errors) == 0 {
		return ""
	}
	var b strings.Builder
	for i, err := range v.errors {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(err.Error())
	}
	return b.String()
}

func (v ValidationErrors) Unwrap() []error {
	return append([]error(nil), v.errors...)
}
