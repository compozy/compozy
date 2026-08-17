package cli

type commandExitError struct {
	code int
	err  error
}

func (e *commandExitError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *commandExitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *commandExitError) cliExitCode() int {
	if e == nil || e.code <= 0 {
		return 1
	}
	return e.code
}

func withCommandExitCode(code int, err error) error {
	if err == nil {
		return nil
	}
	return &commandExitError{code: code, err: err}
}
