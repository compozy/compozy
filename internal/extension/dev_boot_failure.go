package extensionpkg

import (
	"errors"

	"github.com/compozy/compozy/internal/diagnostics"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	devBootDiagnosticMissingOrigin    = "extension development origin is unavailable"
	devBootDiagnosticActivationFailed = "extension development activation failed"
	maxDevBootFailureLogBytes         = 1024
)

func devBootFailureCode(err error) string {
	switch {
	case errors.Is(err, ErrExtensionDevOriginMissing), errors.Is(err, workspacepkg.ErrWorkspaceRootMissing):
		return extensionFailureMissingOrigin
	default:
		return extensionFailureActivationFailed
	}
}

func devBootFailureDiagnostic(code string) string {
	switch code {
	case extensionFailureMissingOrigin:
		return devBootDiagnosticMissingOrigin
	case extensionFailureActivationFailed:
		return devBootDiagnosticActivationFailed
	default:
		return ""
	}
}

func (m *Manager) setDevBootFailure(key InstanceKey, ext *managedExtension, code string, err error) {
	if ext == nil || err == nil {
		return
	}
	diagnostic := devBootFailureDiagnostic(code)
	if diagnostic == "" {
		code = extensionFailureActivationFailed
		diagnostic = devBootDiagnosticActivationFailed
	}
	ext.phase = ExtensionPhase("errored")
	ext.failureCode = code
	ext.lastError = diagnostic
	m.logger.Error(
		"extension.dev.boot.failed",
		managerExtensionKey,
		key.Name,
		"workspace_id",
		key.WorkspaceID,
		"failure_code",
		code,
		"error",
		diagnostics.RedactAndBound(err.Error(), maxDevBootFailureLogBytes),
	)
}
