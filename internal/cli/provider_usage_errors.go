package cli

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
)

// Keep the shared HTTP decoder and give this experimental command its specified
// exit-one and human code suffix, without changing other CLI families.
type providerUsageCommandError struct {
	cause   error
	payload contract.ErrorPayload
}

func (e *providerUsageCommandError) Error() string {
	return fmt.Sprintf("%s (%s)", e.payload.Error, e.payload.Code)
}
func (e *providerUsageCommandError) Unwrap() error                       { return e.cause }
func (*providerUsageCommandError) cliExitCode() int                      { return 1 }
func (e *providerUsageCommandError) errorPayload() contract.ErrorPayload { return e.payload }

func providerUsageCLIError(err error) error {
	if flat, ok := errors.AsType[interface {
		error
		errorPayload() contract.ErrorPayload
	}](err); ok {
		return &providerUsageCommandError{cause: err, payload: flat.errorPayload()}
	}
	if profile, ok := errors.AsType[interface {
		error
		profileErrorPayload() contract.ProfileErrorPayload
	}](err); ok {
		value := profile.profileErrorPayload().Error
		return &providerUsageCommandError{
			cause:   err,
			payload: contract.ErrorPayload{Error: value.Message, Code: value.Code},
		}
	}
	return err
}
