package terminal

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// resolveShell picks the interactive shell: an explicit request, then the
// configured `[terminal].default_shell`, then the platform defaults. Each
// candidate must resolve to an executable; a broken one falls through.
func (m *Service) resolveShell(ctx context.Context, requested, configured string) (string, error) {
	platform := platformShellCandidates(m.platformAccountShell(ctx, requested, configured))
	candidates := append([]string{requested, configured}, platform...)
	seen := make(map[string]struct{}, len(candidates))
	tried := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		tried = append(tried, candidate)
		resolved, err := exec.LookPath(candidate)
		if err == nil {
			return resolved, nil
		}
	}
	return "", &Error{
		Code: ErrorCodeInteractiveUnavailable,
		Message: fmt.Sprintf(
			"no terminal shell is available (tried %s); set [terminal].default_shell to an installed shell",
			strings.Join(tried, ", "),
		),
		Platform: runtime.GOOS,
		Err:      errors.Join(ErrInteractive, exec.ErrNotFound),
	}
}

// platformAccountShell consults the account database only when neither the
// request nor the configuration already names a shell.
func (m *Service) platformAccountShell(ctx context.Context, requested, configured string) string {
	if strings.TrimSpace(requested) != "" || strings.TrimSpace(configured) != "" || m.accountShell == nil {
		return ""
	}
	shell, err := m.accountShell(ctx)
	if err != nil {
		m.logger.Debug("terminal: account login shell unavailable; using inherited SHELL", "error", err)
		return ""
	}
	return shell
}
