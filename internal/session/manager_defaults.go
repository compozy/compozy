package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session/inputqueue"
)

func (m *Manager) applyRuntimeDefaults() error {
	if m.logger == nil {
		m.logger = slog.Default()
	}
	if m.driver == nil {
		return errors.New("session: agent driver is required")
	}
	if m.openStore == nil {
		return errors.New("session: store opener is required")
	}
	if m.lifecycleCtx == nil {
		m.lifecycleCtx = context.Background()
	}
	if m.providerSecrets == nil {
		m.providerSecrets = envProviderSecretResolver{lookupEnv: os.LookupEnv}
	}
	if m.now == nil {
		m.now = func() time.Time {
			return time.Now().UTC()
		}
	}
	if m.newSessionID == nil {
		m.newSessionID = newIDGenerator("sess")
	}
	if m.newSandboxID == nil {
		m.newSandboxID = newIDGenerator("env")
	}
	if m.newTurnID == nil {
		m.newTurnID = newIDGenerator("turn")
	}
	if m.promptBufSize <= 0 {
		m.promptBufSize = defaultPromptBufferSize
	}
	if m.soulLocks == nil {
		m.soulLocks = make(map[string]chan struct{})
	}
	if m.resumeLifecycle.runs == nil {
		m.resumeLifecycle.runs = make(map[string]*sessionResumeRun)
	}
	if m.sessionHealthHookLast == nil {
		m.sessionHealthHookLast = make(map[string]time.Time)
	}
	if m.soulRefreshTimeout <= 0 {
		m.soulRefreshTimeout = defaultLifecycleTimeout
	}
	if m.supervision == (compozyconfig.SessionSupervisionConfig{}) {
		m.supervision = compozyconfig.DefaultSessionSupervisionConfig()
	}
	if err := m.supervision.Validate(); err != nil {
		return fmt.Errorf("session: %w", err)
	}
	if err := m.compaction.Validate(); err != nil {
		return fmt.Errorf("session: %w", err)
	}
	if err := m.applyInputQueueDefaults(); err != nil {
		return err
	}
	if m.sessionHealthStaleAfter <= 0 {
		m.sessionHealthStaleAfter = compozyconfig.DefaultHeartbeatConfig().SessionHealthStaleAfter
	}
	if m.sessionHealthHookMinInterval <= 0 {
		m.sessionHealthHookMinInterval = compozyconfig.DefaultHeartbeatConfig().SessionHealthHookMinInterval
	}
	return nil
}

func (m *Manager) startRuntimeOwners() error {
	if err := m.ensureQueryStoreRuntime(); err != nil {
		return err
	}
	m.processWatchLifecycle.mu.Lock()
	m.ensureProcessWatchContextLocked()
	m.processWatchLifecycle.mu.Unlock()
	return nil
}

func (m *Manager) applyInputQueueDefaults() error {
	m.busyInput = m.busyInput.Normalize()
	if err := m.busyInput.Validate(); err != nil {
		return fmt.Errorf("session: %w", err)
	}
	if m.inputQueueStore == nil {
		return nil
	}
	queue, err := inputqueue.New(
		m.inputQueueStore,
		inputqueue.Config{
			QueueCap:     m.busyInput.QueueCap,
			MaxTextBytes: m.busyInput.MaxTextBytes,
		},
		inputqueue.WithClock(m.now),
		inputqueue.WithIDGenerator(func() (string, error) {
			return newID("inq")
		}),
	)
	if err != nil {
		return fmt.Errorf("session: input queue: %w", err)
	}
	m.inputQueue = queue
	return nil
}
