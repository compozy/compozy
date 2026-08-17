package extensionpkg

import (
	"context"
	"errors"

	"fmt"

	"path/filepath"

	"strings"

	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
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
	return m.shutdownDeadlineForInstance(GlobalInstanceKey(name), generation)
}

func (m *Manager) shutdownDeadlineForInstance(key InstanceKey, generation int64) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ext := m.instanceLocked(key)
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
	normalizedPath := strings.TrimSpace(path)
	if filepath.Base(normalizedPath) == agentPluginManifestFileName {
		return LoadManifest(filepath.Dir(normalizedPath))
	}
	switch strings.ToLower(filepath.Ext(normalizedPath)) {
	case manifestFileExtTOML:
		return loadManifestTOML(normalizedPath)
	case manifestFileExtJSON:
		return loadManifestJSON(normalizedPath)
	default:
		return nil, fmt.Errorf("extension: unsupported manifest path %q", normalizedPath)
	}
}

func shutdownProcessWithTimeout(ctx context.Context, proc processHandle, timeout time.Duration) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if proc == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	shutdownErr := proc.Shutdown(shutdownCtx)
	select {
	case <-proc.Done():
		return errors.Join(shutdownErr, proc.Wait())
	case <-shutdownCtx.Done():
		return errors.Join(shutdownErr, shutdownCtx.Err())
	}
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
		len(manifest.Permissions.Requires) > 0 ||
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
