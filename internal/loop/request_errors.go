package loop

import (
	"errors"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	jsonschemakind "github.com/santhosh-tekuri/jsonschema/v6/kind"
)

// NewRequestReasonError preserves a request sentinel while adding a stable public code.
func NewRequestReasonError(code ReasonCode, err error, details map[string]string) error {
	return reasonError(code, err, details)
}

// NewRequestValidationError projects JSON Schema failures as field-addressed public details.
func NewRequestValidationError(err error) error {
	details := map[string]string{}
	if validation, ok := errors.AsType[*jsonschema.ValidationError](err); ok {
		collectRequestValidationDetails(validation, details)
	}
	if len(details) == 0 {
		details["payload"] = strings.TrimSpace(err.Error())
	}
	return NewRequestReasonError(
		ReasonCodeRequestValidationFailed,
		errors.Join(ErrRequestValidationFailed, err),
		details,
	)
}

func collectRequestValidationDetails(validation *jsonschema.ValidationError, details map[string]string) {
	if validation == nil {
		return
	}
	if len(validation.Causes) > 0 {
		for _, cause := range validation.Causes {
			collectRequestValidationDetails(cause, details)
		}
		return
	}
	if required, ok := validation.ErrorKind.(*jsonschemakind.Required); ok {
		for _, missing := range required.Missing {
			path := append(append([]string(nil), validation.InstanceLocation...), missing)
			details[strings.Join(path, ".")] = validation.Error()
		}
		return
	}
	field := strings.Join(validation.InstanceLocation, ".")
	if field == "" {
		field = payloadFieldKey
	}
	details[field] = validation.Error()
}
