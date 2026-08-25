package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/skills"
)

func renderSkillExposureExecutionError(err error) (string, bool) {
	payload, ok := skillExposureErrorPayload(err)
	if !ok {
		return "", false
	}
	failures := 0
	for _, result := range payload.Results {
		if !result.OK && (result.Error == nil ||
			(result.Error.Code != skills.ExposureCodeRolledBack && result.Error.Code != skills.ExposureCodeNotApplied)) {
			failures++
		}
	}
	count := fmt.Sprintf("%d target", failures)
	if len(payload.Results) > 1 {
		count = fmt.Sprintf("%d of %d targets", failures, len(payload.Results))
	} else if failures != 1 {
		count += "s"
	}
	if payload.RolledBack != nil && *payload.RolledBack {
		count += "; completed targets rolled back"
	}
	title := "Error: skill exposure failed (" + count + ")"
	if createdErr, created := errors.AsType[*skillCreatedExposureError](err); created && createdErr != nil {
		title += " — the skill was created; fix the cause and run `compozy skill expose`"
	}
	lines := []string{title}
	for _, result := range payload.Results {
		if result.OK || result.Error == nil {
			continue
		}
		detail := exposureFailureDetail(*result.Error)
		line := "  " + result.Target + "  " + result.Error.Code
		if detail != "" {
			line += " — " + detail
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), true
}

func exposureFailureDetail(payload contract.SkillExposureErrorPayload) string {
	if payload.OccupiedBy != "" {
		return "occupied by " + payload.OccupiedBy
	}
	return strings.TrimSpace(payload.Message)
}

func marshalSkillExposureExecutionError(args []string, err error) ([]byte, bool) {
	payload, ok := skillExposureErrorPayload(err)
	if !ok {
		return nil, false
	}
	return marshalStructuredPayload(args, payload)
}

func skillExposureErrorPayload(err error) (contract.SkillExposureFailureResponse, bool) {
	apiErr, ok := errors.AsType[interface {
		error
		skillExposureErrorPayload() contract.SkillExposureFailureResponse
	}](err)
	if !ok {
		return contract.SkillExposureFailureResponse{}, false
	}
	return apiErr.skillExposureErrorPayload(), true
}
