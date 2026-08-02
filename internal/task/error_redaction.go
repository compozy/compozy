package task

type redactedTaskError struct {
	message string
	cause   error
}

func (e *redactedTaskError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *redactedTaskError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func redactTaskError(err error) error {
	if err == nil {
		return nil
	}
	message := redactTaskSecretText(err.Error())
	if message == err.Error() {
		return err
	}
	return &redactedTaskError{message: message, cause: err}
}
