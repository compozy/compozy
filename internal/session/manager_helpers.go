package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) startPermissions(configured string) compozyconfig.PermissionMode {
	mode := compozyconfig.PermissionMode(strings.TrimSpace(configured))
	if mode == "" {
		return compozyconfig.PermissionModeApproveReads
	}
	return mode
}

func (m *Manager) writeMeta(session *Session) error {
	if session == nil {
		return errors.New("session: session is required")
	}
	meta := session.meta()
	if err := store.WriteSessionMeta(session.MetaPath(), &meta); err != nil {
		return fmt.Errorf("session: write meta for %q: %w", session.ID, err)
	}
	return nil
}

func (m *Manager) activateAndWatch(
	ctx context.Context,
	session *Session,
	proc *AgentProcess,
	adoptCurrentModel bool,
	resolved compozyconfig.ResolvedAgent,
	postEvent hookspkg.HookEvent,
	preserveStopReason bool,
) error {
	now := m.now()
	if err := m.activate(session); err != nil {
		return err
	}
	if err := session.activateWithProcess(proc, now, adoptCurrentModel, preserveStopReason); err != nil {
		return err
	}
	if err := m.persistSessionLifecycleState(ctx, session, true); err != nil {
		rollbackErr := m.rollbackActivation(session, proc, now)
		return errors.Join(err, rollbackErr)
	}

	m.dispatchAgentSpawned(ctx, session, proc, resolved)
	switch postEvent {
	case hookspkg.HookSessionPostCreate:
		m.dispatchSessionPostCreate(ctx, session)
	case hookspkg.HookSessionPostResume:
		m.dispatchSessionPostResume(ctx, session)
	}
	if m.notifier != nil {
		m.notifier.OnSessionCreated(ctx, session)
	}
	if _, err := m.persistSessionPresence(ctx, session, now); err != nil {
		m.sessionLogger(session).Warn("session: persist health presence failed", "error", err)
	}
	m.watchProcess(session)
	return nil
}

func (m *Manager) rollbackActivation(session *Session, proc *AgentProcess, now time.Time) error {
	if session == nil {
		return nil
	}

	m.remove(session.ID)
	session.rollbackActivation(now)

	if proc == nil {
		return nil
	}

	stopCtx, cancel := m.lifecycleCleanupContext()
	defer cancel()
	return m.driver.Stop(stopCtx, proc)
}

// sessionLogger identifies the bound route, which can differ from an attempted replacement route.
func (m *Manager) sessionLogger(session *Session) *slog.Logger {
	logger := m.logger
	if logger == nil {
		logger = slog.Default()
	}
	if session == nil {
		return logger
	}

	info := session.Info()
	return logger.With("session_id", info.ID, "agent_name", info.AgentName, "provider", info.Provider,
		"provider_command_fingerprint", providerCommandFingerprint(session.providerRoutingSnapshot().Command))
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func isProcessDone(proc *AgentProcess) bool {
	if proc == nil {
		return true
	}
	select {
	case <-proc.Done():
		return true
	default:
		return false
	}
}

func waitForPromptSetup(ctx context.Context, session *Session, promptSetupDone <-chan struct{}) error {
	if promptSetupDone == nil {
		return nil
	}
	select {
	case <-promptSetupDone:
		return nil
	case <-ctx.Done():
		sessionID := ""
		if session != nil {
			sessionID = session.ID
		}
		return fmt.Errorf("session: wait for in-flight prompt setup for %q: %w", sessionID, ctx.Err())
	}
}

func newID(prefix string) (string, error) {
	return newIDFromReader(prefix, rand.Reader)
}

func newIDFromReader(prefix string, entropy io.Reader) (string, error) {
	if entropy == nil {
		return "", errors.New("session: id entropy source is required")
	}

	var random [8]byte
	if _, err := io.ReadFull(entropy, random[:]); err != nil {
		return "", fmt.Errorf("session: read id entropy: %w", err)
	}

	suffix := hex.EncodeToString(random[:])
	if strings.TrimSpace(prefix) == "" {
		return suffix, nil
	}
	return prefix + "-" + suffix, nil
}

func newIDGenerator(prefix string) IDGenerator {
	return func() (string, error) {
		return newID(prefix)
	}
}
