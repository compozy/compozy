package cli

import (
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/modelcatalog"
)

func formatCostProvenance(totalCost *float64, currency string, status contract.CostStatus) string {
	switch status {
	case modelcatalog.CostStatusIncluded:
		return "Included"
	case modelcatalog.CostStatusUnknown:
		return "Unknown"
	case modelcatalog.CostStatusActual, modelcatalog.CostStatusEstimated:
	default:
		return ""
	}
	if totalCost == nil {
		return ""
	}
	amount := formatFloat64Ptr(totalCost)
	if currency = strings.TrimSpace(currency); currency != "" {
		amount = currency + " " + amount
	}
	if status == modelcatalog.CostStatusEstimated {
		return amount + " (estimated)"
	}
	return amount
}

func formatInt64Ptr(value *int64) string {
	if value == nil {
		return ""
	}
	return int64OrDash(*value)
}

func formatFloat64Ptr(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}

func formatStringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
