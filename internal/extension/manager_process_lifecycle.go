package extensionpkg

import (
	"context"

	"fmt"

	"path/filepath"

	"strings"

	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func (m *Manager) waitBackoff(delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-m.lifecycleDone():
		return false
	}
}

func (m *Manager) lifecycleDone() <-chan struct{} {
	m.mu.RLock()
	ctx := m.lifecycleCtx
	m.mu.RUnlock()

	if ctx == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return ctx.Done()
}

func (m *Manager) lifecycleContext() context.Context {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.lifecycleCtx == nil {
		return context.Background()
	}
	return m.lifecycleCtx
}

func (m *Manager) healthPollInterval(healthInterval time.Duration) time.Duration {
	if healthInterval <= 0 {
		return m.healthPollCeiling
	}
	interval := min(max(healthInterval/4, m.healthPollFloor), m.healthPollCeiling)
	return interval
}

func (m *Manager) shutdownDeadlineForProcess(name string, generation int64) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ext := m.extensions[name]
	if ext == nil || ext.generation != generation || ext.runtime.ShutdownTimeoutMS <= 0 {
		return m.defaultShutdownTimeout
	}
	return time.Duration(ext.runtime.ShutdownTimeoutMS) * time.Millisecond
}

func restartBackoff(failures int, maximum time.Duration) time.Duration {
	if failures <= 0 {
		return 0
	}
	delay := time.Second << (failures - 1)
	if delay > maximum {
		return maximum
	}
	return delay
}

func loadManifestAtPath(path string) (*Manifest, error) {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(path))) {
	case manifestFileExtTOML:
		return loadManifestTOML(path)
	case manifestFileExtJSON:
		return loadManifestJSON(path)
	default:
		return nil, fmt.Errorf("extension: unsupported manifest path %q", path)
	}
}

func shutdownProcessWithTimeout(ctx context.Context, proc processHandle, timeout time.Duration) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if proc == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return proc.Shutdown(shutdownCtx)
}

func phaseError(name string, phase ExtensionPhase, err error) error {
	return fmt.Errorf("extension %q %s: %w", name, phase, err)
}

func requiresSubprocess(manifest *Manifest) bool {
	if manifest == nil {
		return false
	}
	if strings.TrimSpace(manifest.Subprocess.Command) != "" {
		return true
	}
	return len(manifest.Capabilities.Provides) > 0 ||
		len(manifest.Actions.Requires) > 0 ||
		len(manifest.Resources.Publish.Families) > 0
}

func durationOr(value Duration, fallback time.Duration) time.Duration {
	if value.IsZero() {
		return fallback
	}
	return time.Duration(value)
}

func validateSupportedHookEvents(values []string) error {
	for _, value := range values {
		if err := hookspkg.HookEvent(strings.TrimSpace(value)).Validate(); err != nil {
			return err
		}
	}
	return nil
}
