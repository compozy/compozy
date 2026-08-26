package daemon

import (
	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func configApplyFailure(
	subsystem string,
	category string,
	summary string,
	err error,
) settingspkg.ApplyFailure {
	return settingspkg.ApplyFailure{
		Subsystem: subsystem,
		Diagnostic: diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            "config.apply." + subsystem + "_sync_failed",
			Code:          diagnosticcontract.CodeConfigPartialFailure,
			Category:      category,
			Title:         summary,
			Message:       diagnostics.RedactAndBound(err.Error(), 1024),
			Severity:      diagnosticcontract.SeverityError,
			DataFreshness: diagnosticcontract.FreshnessLive,
		},
			diagnostics.WithSuggestedCommand("compozy config reload"),
		),
	}
}
