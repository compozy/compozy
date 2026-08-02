package extensionpkg

import (
	"fmt"
	"strings"
)

type resourceValidationError struct {
	Path string
	Err  error
}

func (e *resourceValidationError) Error() string {
	if e == nil {
		return "extension: invalid static resource"
	}
	return fmt.Sprintf("extension: invalid static resource %q: %v", strings.TrimSpace(e.Path), e.Err)
}

func (e *resourceValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func wrapResourceValidationError(path string, err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*resourceValidationError); ok {
		return err
	}
	return &resourceValidationError{Path: strings.TrimSpace(path), Err: err}
}
