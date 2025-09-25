package core

type Error struct {
	Message string         `json:"message,omitempty"`
	Code    string         `json:"code,omitempty"`
	Details map[string]any `json:"details,omitempty"`
	cause   error
}

func NewError(err error, code string, details map[string]any) *Error {
	var message string
	if err != nil {
		message = err.Error()
	} else {
		message = "unknown error"
	}
	return &Error{
		Message: message,
		Code:    code,
		Details: details,
		cause:   err,
	}
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *Error) AsMap() map[string]any {
	if e == nil {
		return nil
	}

	// Return nil if the error has no meaningful content
	if e.Message == "" && e.Code == "" && e.Details == nil {
		return nil
	}

	return map[string]any{
		"message": e.Message,
		"code":    e.Code,
		"details": e.Details,
	}
}
