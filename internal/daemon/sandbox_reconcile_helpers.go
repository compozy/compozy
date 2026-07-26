package daemon

import (
	"encoding/json"

	"log/slog"

	"strings"
	"time"

	"github.com/compozy/agh/internal/sandbox"

	"github.com/compozy/agh/internal/store"
)

func mergeSandboxSessionState(
	base sandbox.SessionState,
	overlay sandbox.SessionState,
) sandbox.SessionState {
	next := base
	if strings.TrimSpace(overlay.SandboxID) != "" {
		next.SandboxID = strings.TrimSpace(overlay.SandboxID)
	}
	if overlay.Backend.Valid() {
		next.Backend = overlay.Backend
	}
	if strings.TrimSpace(overlay.Profile) != "" {
		next.Profile = strings.TrimSpace(overlay.Profile)
	}
	if strings.TrimSpace(overlay.State) != "" {
		next.State = strings.TrimSpace(overlay.State)
	}
	if strings.TrimSpace(overlay.InstanceID) != "" {
		next.InstanceID = strings.TrimSpace(overlay.InstanceID)
	}
	if strings.TrimSpace(overlay.RuntimeRootDir) != "" {
		next.RuntimeRootDir = strings.TrimSpace(overlay.RuntimeRootDir)
	}
	if len(overlay.RuntimeAdditionalDirs) > 0 {
		next.RuntimeAdditionalDirs = append([]string(nil), overlay.RuntimeAdditionalDirs...)
	}
	if len(overlay.ProviderState) > 0 {
		next.ProviderState = cloneDaemonRawMessage(overlay.ProviderState)
	}
	if overlay.SSHAccessExpiresAt != nil {
		next.SSHAccessExpiresAt = cloneDaemonTimePointer(overlay.SSHAccessExpiresAt)
	}
	if !overlay.PreparedAt.IsZero() {
		next.PreparedAt = overlay.PreparedAt
	}
	return next
}

func sandboxReconcileLogAttrs(
	meta store.SessionMeta,
	envMeta *store.SessionSandboxMeta,
	duration time.Duration,
	err error,
) []any {
	attrs := []any{
		daemonPayloadSessionIDKey, strings.TrimSpace(meta.ID),
		sandboxReconcileWorkspaceIDKey, strings.TrimSpace(meta.WorkspaceID),
		"session_state", strings.TrimSpace(meta.State),
		"duration_ms", duration.Milliseconds(),
	}
	if envMeta != nil {
		attrs = append(
			attrs,
			"backend", strings.TrimSpace(envMeta.Backend),
			"profile", strings.TrimSpace(envMeta.Profile),
			"sandbox_id", strings.TrimSpace(envMeta.SandboxID),
			"instance_id", strings.TrimSpace(envMeta.InstanceID),
		)
	}
	if err != nil {
		attrs = append(attrs, "error", err)
	}
	return attrs
}

func sandboxReconcileLogger(state *bootState) *slog.Logger {
	if state != nil && state.logger != nil {
		return state.logger
	}
	return slog.Default()
}

func cloneDaemonSessionSandboxMeta(
	meta *store.SessionSandboxMeta,
) *store.SessionSandboxMeta {
	if meta == nil {
		return nil
	}
	cloned := *meta
	cloned.RuntimeAdditionalDirs = append([]string(nil), meta.RuntimeAdditionalDirs...)
	cloned.ProviderState = cloneDaemonRawMessage(meta.ProviderState)
	cloned.SSHAccessExpiresAt = cloneDaemonTimePointer(meta.SSHAccessExpiresAt)
	cloned.LastSyncAt = cloneDaemonTimePointer(meta.LastSyncAt)
	return &cloned
}

func cloneDaemonRawMessage(value json.RawMessage) json.RawMessage {
	if value == nil {
		return nil
	}
	cloned := make(json.RawMessage, len(value))
	copy(cloned, value)
	return cloned
}

func cloneDaemonTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneDaemonStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := strings.TrimSpace(*value)
	return &cloned
}
