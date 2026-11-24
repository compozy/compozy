package core

import (
	"cmp"
	"errors"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/admission"
	"github.com/compozy/compozy/internal/api/contract"
)

func workAdmissionErrorPayload(err error) *contract.ErrorPayload {
	switch {
	case errors.Is(err, account.ErrSignInRequired):
		return &contract.ErrorPayload{Code: account.ErrSignInRequired.Code, Error: account.ErrSignInRequired.Message}
	case errors.Is(err, admission.ErrStarting):
		return &contract.ErrorPayload{
			Code: cmp.Or(
				DeriveErrorCode(err),
				admission.StartingCode,
			),
			Error:     admission.ErrStarting.Error(),
			Retryable: true,
		}
	case errors.Is(err, admission.ErrLocalExecutionDisabled):
		return &contract.ErrorPayload{
			Code: admission.LocalExecutionDisabledCode, Error: admission.LocalExecutionDisabledMessage,
		}
	default:
		return nil
	}
}
