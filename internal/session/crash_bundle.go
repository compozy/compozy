package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/fileutil"
	"github.com/compozy/agh/internal/store"
)

const (
	crashBundleDirName      = "crash-bundles"
	crashBundleSchema       = "agh.session_crash_bundle.v1"
	maxCrashEvidenceBytes   = 32 << 10
	crashBundleFileMode     = 0o600
	crashBundleDirMode      = 0o700
	crashBundleNameMaxBytes = 160
	crashBundleUnknownName  = "unknown"
)

type crashBundleDocument struct {
	Schema    string               `json:"schema"`
	SessionID string               `json:"session_id"`
	AgentName string               `json:"agent_name,omitempty"`
	Provider  string               `json:"provider,omitempty"`
	State     State                `json:"state,omitempty"`
	Failure   store.SessionFailure `json:"failure"`
	Process   *crashBundleProcess  `json:"process,omitempty"`
	Error     string               `json:"error,omitempty"`
	Stderr    string               `json:"stderr,omitempty"`
	CreatedAt time.Time            `json:"created_at"`
}

type crashBundleProcess struct {
	PID       int       `json:"pid,omitempty"`
	Command   string    `json:"command,omitempty"`
	Args      []string  `json:"args,omitempty"`
	Cwd       string    `json:"cwd,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

func (m *Manager) attachCrashBundleToFailure(
	ctx context.Context,
	session *Session,
	failure *store.SessionFailure,
	err error,
	stderr string,
) (*store.SessionFailure, error) {
	if failure == nil || !failureShouldHaveCrashBundle(failure.Kind) {
		return store.CloneSessionFailure(failure), nil
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return store.CloneSessionFailure(failure), err
		}
	}

	normalized := normalizeSessionFailure(failure, "")
	if normalized == nil {
		return nil, nil
	}
	path, err := m.writeCrashBundle(session, *normalized, err, stderr)
	if err != nil {
		return normalized, err
	}
	normalized.CrashBundlePath = path
	return normalized, nil
}

func (m *Manager) attachPromptFailureDiagnostics(
	ctx context.Context,
	session *Session,
	event acp.AgentEvent,
) acp.AgentEvent {
	if event.Failure == nil {
		return event
	}
	var eventErr error
	if strings.TrimSpace(event.Error) != "" {
		eventErr = fmt.Errorf("%s", event.Error)
	}
	failure, err := m.attachCrashBundleToFailure(ctx, session, event.Failure, eventErr, "")
	if err != nil {
		m.sessionLogger(session).Warn(
			"session: write prompt failure crash bundle failed",
			"turn_id", event.TurnID,
			"failure_kind", event.Failure.Kind,
			"error", err,
		)
	}
	event.Failure = failure
	event.Error = failureSummary(failure, event.Error)
	return event
}

func (m *Manager) writeCrashBundle(
	session *Session,
	failure store.SessionFailure,
	err error,
	stderr string,
) (string, error) {
	if session == nil {
		return "", nil
	}
	info := session.Info()
	if info == nil {
		return "", nil
	}
	dir := filepath.Join(m.homePaths.LogsDir, crashBundleDirName)
	if err := os.MkdirAll(dir, crashBundleDirMode); err != nil {
		return "", fmt.Errorf("session: create crash bundle directory %q: %w", dir, err)
	}

	normalizedFailure := normalizeSessionFailure(&failure, "")
	if normalizedFailure == nil {
		return "", nil
	}
	document := crashBundleDocument{
		Schema:    crashBundleSchema,
		SessionID: strings.TrimSpace(info.ID),
		AgentName: strings.TrimSpace(info.AgentName),
		Provider:  strings.TrimSpace(info.Provider),
		State:     info.State,
		Failure:   *normalizedFailure,
		Error:     diagnostics.RedactAndBound(errorText(err), maxCrashEvidenceBytes),
		Stderr:    diagnostics.RedactAndBound(stderr, maxCrashEvidenceBytes),
		CreatedAt: m.now().UTC(),
	}
	if proc := session.processHandle(); proc != nil {
		document.Process = &crashBundleProcess{
			PID:       proc.PID,
			Command:   diagnostics.RedactAndBound(proc.Command, maxCrashEvidenceBytes),
			Args:      redactStringSlice(proc.Args),
			Cwd:       diagnostics.RedactAndBound(proc.Cwd, maxCrashEvidenceBytes),
			StartedAt: proc.StartedAt.UTC(),
		}
	}

	payload, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return "", fmt.Errorf("session: marshal crash bundle: %w", err)
	}
	payload = append(payload, '\n')

	path := filepath.Join(dir, crashBundleFileName(info.ID, failure.Kind, document.CreatedAt))
	if err := fileutil.AtomicWriteFile(path, payload, crashBundleFileMode); err != nil {
		return "", fmt.Errorf("session: write crash bundle %q: %w", path, err)
	}
	return path, nil
}

func failureShouldHaveCrashBundle(kind store.FailureKind) bool {
	switch kind {
	case store.FailureStartup,
		store.FailureHandshake,
		store.FailureLoad,
		store.FailureProtocol,
		store.FailurePrompt,
		store.FailureProcess,
		store.FailureTransport:
		return true
	default:
		return false
	}
}

func crashBundleFileName(sessionID string, kind store.FailureKind, ts time.Time) string {
	sessionName := sanitizeCrashBundleName(sessionID)
	kindName := sanitizeCrashBundleName(string(kind))
	suffix := kindName + "-" + fmt.Sprintf("%d", ts.UnixNano())
	maxSessionNameBytes := crashBundleNameMaxBytes - len(suffix) - 1
	if maxSessionNameBytes > 0 && len(sessionName) > maxSessionNameBytes {
		sessionName = strings.Trim(sessionName[:maxSessionNameBytes], "-_")
		if sessionName == "" {
			sessionName = crashBundleUnknownName
		}
	}

	base := sessionName + "-" + suffix
	if len(base) > crashBundleNameMaxBytes {
		base = base[len(base)-crashBundleNameMaxBytes:]
	}
	return base + ".json"
}

func sanitizeCrashBundleName(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return crashBundleUnknownName
	}
	var builder strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}
	sanitized := strings.Trim(builder.String(), "-")
	if sanitized == "" {
		return crashBundleUnknownName
	}
	return sanitized
}

func redactStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	redacted := make([]string, 0, len(values))
	for _, value := range values {
		redacted = append(redacted, diagnostics.RedactAndBound(value, maxCrashEvidenceBytes))
	}
	return redacted
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
