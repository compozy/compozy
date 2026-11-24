package computers

import (
	"errors"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
)

func describe(err error) *Failure {
	if failure, ok := errors.AsType[*Failure](err); ok {
		return failure
	}
	if failure, ok := errors.AsType[*cloudclient.Failure](err); ok {
		code := failure.Code
		if code == "reauth_required" && failure.Reason == CodeDeviceRevoked {
			code = CodeDeviceRevoked
		}
		return &Failure{Status: failure.Status, Code: code, Message: failure.Message}
	}
	return &Failure{Status: http.StatusServiceUnavailable, Code: "computer_unavailable",
		Message: "The computer is unavailable."}
}

func activeFailure(err error) routeProblem {
	step := "computer"
	switch describe(err).Code {
	case CodeLeaseExpired, CodePlanRequired:
		step = "cloud"
	case CodeDeviceRevoked, "sign_in_required":
		step = "sign_in"
	case ErrOffline.Code:
		step = "route"
	}
	return routeProblem{step: step, err: err}
}

func diagnose(problem routeProblem, at time.Time) (Status, *Diagnosis) {
	failure := describe(problem.err)
	diagnosis := &Diagnosis{Step: problem.step, Code: failure.Code, Message: failure.Message, At: at}
	switch failure.Code {
	case ErrPlanRequired.Code:
		return NeedsPro, diagnosis
	case ErrIncompatible.Code:
		return Incompatible, diagnosis
	case ErrOffline.Code:
		return Offline, diagnosis
	case ErrDisabled.Code:
		return Disabled, diagnosis
	default:
		return NeedsAttention, diagnosis
	}
}

func sameDiagnosis(a, b *Diagnosis) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Step == b.Step && a.Code == b.Code && a.Message == b.Message
}
