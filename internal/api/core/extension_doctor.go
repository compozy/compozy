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
	for _, extension := range extensions {
		name := strings.TrimSpace(extension.Name)
		if extension.ConsecutiveFailures >= extensionpkg.DefaultRestartFailureThreshold {
			items = append(items, diagnostics.NewItem(
				"doctor.extension."+name+".crash_loop",
				contract.CodeExtensionRuntimeUnavailable,
				contract.CategoryExtension,
				"Extension is crash-looping",
				fmt.Sprintf(
					"Extension %q reached %d consecutive failures and is no longer restarting.",
					name,
					extension.ConsecutiveFailures,
				),
				contract.SeverityError,
				contract.FreshnessLive,
				diagnostics.WithSuggestedCommand("compozy extension status "+name),
				diagnostics.WithEvidence(map[string]any{
					extensionDoctorNameEvidenceKey: name,
					"consecutive_failures":         extension.ConsecutiveFailures,
					"restart_backoff_ms":           extension.RestartBackoffMS,
				}),
			))
		}
		if len(extension.MissingEnv) > 0 {
			items = append(items, diagnostics.NewItem(
				"doctor.extension."+name+".missing_env",
				contract.CodeExtensionRuntimeUnavailable,
				contract.CategoryExtension,
				"Extension environment is incomplete",
				fmt.Sprintf(
					"Extension %q is missing required environment variables: %s.",
					name,
					strings.Join(extension.MissingEnv, ", "),
				),
				contract.SeverityError,
				contract.FreshnessLive,
				diagnostics.WithSuggestedCommand(
					"compozy extension secrets set "+name+" --env "+strings.TrimSpace(extension.MissingEnv[0]),
				),
				diagnostics.WithEvidence(map[string]any{
					extensionDoctorNameEvidenceKey: name,
					"missing_env":                  append([]string(nil), extension.MissingEnv...),
				}),
			))
		}
		if extension.Dev && strings.TrimSpace(extension.FailureCode) == "missing_origin" {
			items = append(items, diagnostics.NewItem(
				"doctor.extension."+name+".stale_dev_origin",
				contract.CodeExtensionRuntimeUnavailable,
				contract.CategoryExtension,
				"Extension development origin is stale",
				fmt.Sprintf("Extension %q points to a development origin that no longer exists.", name),
				contract.SeverityError,
				contract.FreshnessLive,
				diagnostics.WithSuggestedCommand("compozy extension dev "+extension.OriginPath),
				diagnostics.WithEvidence(map[string]any{
					extensionDoctorNameEvidenceKey: name,
					"origin_path":                  extension.OriginPath,
				}),
			))
		}
	}
	return items
}
