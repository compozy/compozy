package cli

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

type marketplaceSourceAPIError struct {
	statusCode int
	status     string
	payload    contract.MarketplaceSourceErrorPayload
}

func (e *marketplaceSourceAPIError) Error() string {
	message := apiErrorMessage(e.payload.Error, e.status)
	if e.payload.SuggestedName != "" {
		message += "; suggested name: " + e.payload.SuggestedName
	}
	if len(e.payload.Checked) > 0 {
		message += "; checked: " + strings.Join(e.payload.Checked, ", ")
	}
	if len(e.payload.RetainedBy) > 0 {
		message += "; retained by: " + strings.Join(e.payload.RetainedBy, ", ")
	}
	return message
}

func (e *marketplaceSourceAPIError) cliExitCode() int {
	if e.statusCode == http.StatusUnprocessableEntity || e.statusCode == http.StatusConflict {
		return 2
	}
	return apiStatusExitCode(e.statusCode)
}

func parseMarketplaceSourceAPIError(statusCode int, status string, body []byte) (bool, error) {
	var payload contract.MarketplaceSourceErrorPayload
	if json.Unmarshal(body, &payload) != nil {
		return false, nil
	}
	switch payload.Code {
	case "marketplace_source_exists", "marketplace_not_a_marketplace", "marketplace_document_too_large",
		"marketplace_source_invalid_ref", "marketplace_source_preset_readonly", "marketplace_source_not_found",
		"marketplace_source_name_retained", "marketplace_source_name_reserved", "marketplace_source_name_invalid",
		"source_unreachable":
		payload.Error = redactToolDiagnostic(payload.Error)
		return true, &marketplaceSourceAPIError{statusCode: statusCode, status: status, payload: payload}
	default:
		return false, nil
	}
}

func marshalMarketplaceSourceExecutionError(args []string, err error) ([]byte, bool) {
	sourceErr, ok := errors.AsType[*marketplaceSourceAPIError](err)
	if !ok {
		return nil, false
	}
	return marshalStructuredPayload(args, sourceErr.payload)
}
