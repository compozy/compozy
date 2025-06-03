package core

type Error struct {
	Message string         `json:"message,omitempty"`
	Code    string         `json:"code,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func NewError(err error, code string, details map[string]any) *Error {
	return &Error{
		Message: err.Error(),
		Code:    code,
		Details: details,
	}
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
