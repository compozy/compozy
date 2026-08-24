package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/doctor"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

const extensionDoctorNameEvidenceKey = "extension_name"

func (h *BaseHandlers) registerExtensionDoctorProbe(registry *doctor.Registry) error {
	if h.Extensions == nil {
		return nil
	}
	return registry.Register(&doctor.ProbeFunc{
		ProbeID:       "runtime.extensions",
		ProbeCategory: contract.CategoryExtension,
		RunFunc: func(ctx context.Context, _ *doctor.ProbeEnv) ([]contract.DiagnosticItem, error) {
			extensions, err := h.Extensions.List(ctx)
			if err != nil {
				return nil, err
			}
			return extensionDiagnosticItems(extensions), nil
		},
	})
}

func extensionDiagnosticItems(extensions []contract.ExtensionPayload) []contract.DiagnosticItem {
	items := make([]contract.DiagnosticItem, 0)
	for index := range extensions {
		extension := &extensions[index]
		name := strings.TrimSpace(extension.Name)
		failureCode := strings.TrimSpace(extension.FailureCode)
		if extension.ConsecutiveFailures >= extensionpkg.DefaultRestartFailureThreshold {
			items = append(items, diagnostics.NewItem(diagnostics.ItemSpec{
				ID:       "doctor.extension." + name + ".crash_loop",
				Code:     contract.CodeExtensionRuntimeUnavailable,
				Category: contract.CategoryExtension,
				Title:    "Extension is crash-looping",
				Message: fmt.Sprintf(
					"Extension %q reached %d consecutive failures and is no longer restarting.",
					name,
					extension.ConsecutiveFailures,
				),
				Severity:      contract.SeverityError,
				DataFreshness: contract.FreshnessLive,
			},
				diagnostics.WithSuggestedCommand("compozy extension status "+name),
				diagnostics.WithEvidence(map[string]any{
					extensionDoctorNameEvidenceKey: name,
					"consecutive_failures":         extension.ConsecutiveFailures,
					"restart_backoff_ms":           extension.RestartBackoffMS,
				}),
			))
		}
		if len(extension.MissingEnv) > 0 {
			items = append(items, diagnostics.NewItem(diagnostics.ItemSpec{
				ID:       "doctor.extension." + name + ".missing_env",
				Code:     contract.CodeExtensionRuntimeUnavailable,
				Category: contract.CategoryExtension,
				Title:    "Extension environment is incomplete",
				Message: fmt.Sprintf(
					"Extension %q is missing required environment variables: %s.",
					name,
					strings.Join(extension.MissingEnv, ", "),
				),
				Severity:      contract.SeverityError,
				DataFreshness: contract.FreshnessLive,
			},
				diagnostics.WithSuggestedCommand(
					"compozy extension secrets set "+name+" --env "+strings.TrimSpace(extension.MissingEnv[0]),
				),
				diagnostics.WithEvidence(map[string]any{
					extensionDoctorNameEvidenceKey: name,
					"missing_env":                  append([]string(nil), extension.MissingEnv...),
				}),
			))
		}
		if extension.Dev && failureCode == "missing_origin" {
			items = append(items, diagnostics.NewItem(diagnostics.ItemSpec{
				ID:            "doctor.extension." + name + ".stale_dev_origin",
				Code:          contract.CodeExtensionRuntimeUnavailable,
				Category:      contract.CategoryExtension,
				Title:         "Extension development origin is stale",
				Message:       fmt.Sprintf("Extension %q points to a development origin that no longer exists.", name),
				Severity:      contract.SeverityError,
				DataFreshness: contract.FreshnessLive,
			},
				diagnostics.WithSuggestedCommand("compozy extension status "+name),
				diagnostics.WithEvidence(map[string]any{
					extensionDoctorNameEvidenceKey: name,
					"failure_code":                 failureCode,
				}),
			))
		}
	}
	return items
}
