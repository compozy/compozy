package knowledge

import "strings"

type ValidationErrors struct {
	errors []error
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
