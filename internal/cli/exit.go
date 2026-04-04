package cli

type commandExitError struct {
	code int
	err  error
}

func (e *commandExitError) Error() string {
	return e.err.Error()
}

func (e *commandExitError) Unwrap() error {
	return e.err
}

func (e *commandExitError) ExitCode() int {
	return e.code
}

func withExitCode(code int, err error) error {
	if err == nil {
		return nil
	}
	return &commandExitError{code: code, err: err}
}
