// Package providertest contains reusable provider conformance checks.
package providertest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/compozy/agh/internal/sandbox"
)

// LifecycleCase configures the shared Provider lifecycle compliance suite.
type LifecycleCase struct {
	Provider         sandbox.Provider
	Backend          sandbox.Backend
	PrepareRequest   sandbox.PrepareRequest
	AssertPrepared   func(*testing.T, sandbox.Prepared)
	AssertFinalState func(*testing.T, sandbox.SessionState)
}

// RunLifecycle exercises the common prepare, sync-to, sync-from, destroy lifecycle.
func RunLifecycle(t *testing.T, tc LifecycleCase) sandbox.Prepared {
	t.Helper()

	assertPrepared := func(prepared sandbox.Prepared) {
		if tc.AssertPrepared != nil {
			tc.AssertPrepared(t, prepared)
		}
	}
	prepared, err := runLifecycleWithPreparedAssertion(context.Background(), tc, assertPrepared, func(err error) {
		t.Errorf("%v", err)
	})
	if err != nil {
		t.Fatal(err)
	}
	if tc.AssertFinalState != nil {
		tc.AssertFinalState(t, prepared.State)
	}

	return prepared
}

func runLifecycle(ctx context.Context, tc LifecycleCase) (sandbox.Prepared, error) {
	return runLifecycleWithPreparedAssertion(ctx, tc, nil, nil)
}

func runLifecycleWithPreparedAssertion(
	ctx context.Context,
	tc LifecycleCase,
	assertPrepared func(sandbox.Prepared),
	reportCleanupError func(error),
) (prepared sandbox.Prepared, err error) {
	if tc.Provider == nil {
		return sandbox.Prepared{}, errors.New("provider = nil, want provider")
	}
	if tc.Backend != "" {
		if got := tc.Provider.Backend(); got != tc.Backend {
			return sandbox.Prepared{}, fmt.Errorf("Provider.Backend() = %q, want %q", got, tc.Backend)
		}
	}

	prepared, err = tc.Provider.Prepare(ctx, tc.PrepareRequest)
	if err != nil {
		return sandbox.Prepared{}, fmt.Errorf("Provider.Prepare() error = %w", err)
	}
	cleanupState := prepared.State
	needsDestroy := true
	defer func() {
		if !needsDestroy {
			return
		}
		destroyErr := tc.Provider.Destroy(ctx, cleanupState)
		if destroyErr != nil {
			wrapped := fmt.Errorf("Provider.Destroy() error = %w", destroyErr)
			if err != nil {
				err = errors.Join(err, wrapped)
				return
			}
			if reportCleanupError != nil {
				reportCleanupError(wrapped)
				return
			}
			err = wrapped
		}
	}()
	if prepared.Launcher == nil {
		return sandbox.Prepared{}, errors.New("Prepared.Launcher = nil, want launcher")
	}
	if prepared.ToolHost == nil {
		return sandbox.Prepared{}, errors.New("Prepared.ToolHost = nil, want tool host")
	}
	if assertPrepared != nil {
		assertPrepared(prepared)
	}

	if _, err := tc.Provider.SyncToRuntime(ctx, prepared.State, sandbox.SyncOptions{
		Reason: sandbox.SyncReasonStart,
	}); err != nil {
		return sandbox.Prepared{}, fmt.Errorf("Provider.SyncToRuntime() error = %w", err)
	}
	if _, err := tc.Provider.SyncFromRuntime(ctx, prepared.State, sandbox.SyncOptions{
		Reason: sandbox.SyncReasonStop,
	}); err != nil {
		return sandbox.Prepared{}, fmt.Errorf("Provider.SyncFromRuntime() error = %w", err)
	}
	needsDestroy = false
	if err := tc.Provider.Destroy(ctx, prepared.State); err != nil {
		return sandbox.Prepared{}, fmt.Errorf("Provider.Destroy() error = %w", err)
	}

	return prepared, nil
}
