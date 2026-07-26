package globaldb

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/testutil"
)

func TestAppMetadataFreshDB(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	ctx := testutil.Context(t)

	t.Run("Should report missing key as not found", func(t *testing.T) {
		value, found, err := db.GetAppMetadata(ctx, "onboarding.completed")
		if err != nil {
			t.Fatalf("GetAppMetadata() error = %v", err)
		}
		if found {
			t.Fatalf("GetAppMetadata() found = true, want false")
		}
		if value != "" {
			t.Fatalf("GetAppMetadata() value = %q, want empty", value)
		}
	})

	t.Run("Should upsert and read back a value", func(t *testing.T) {
		if err := db.SetAppMetadata(ctx, "onboarding.completed", "true"); err != nil {
			t.Fatalf("SetAppMetadata() error = %v", err)
		}
		value, found, err := db.GetAppMetadata(ctx, "onboarding.completed")
		if err != nil {
			t.Fatalf("GetAppMetadata() error = %v", err)
		}
		if !found || value != "true" {
			t.Fatalf("GetAppMetadata() = (%q, %v), want (\"true\", true)", value, found)
		}
	})

	t.Run("Should overwrite an existing key", func(t *testing.T) {
		if err := db.SetAppMetadata(ctx, "onboarding.completed", "false"); err != nil {
			t.Fatalf("SetAppMetadata() error = %v", err)
		}
		value, _, err := db.GetAppMetadata(ctx, "onboarding.completed")
		if err != nil {
			t.Fatalf("GetAppMetadata() error = %v", err)
		}
		if value != "false" {
			t.Fatalf("GetAppMetadata() value = %q, want \"false\"", value)
		}
	})

	t.Run("Should reject blank keys", func(t *testing.T) {
		if err := db.SetAppMetadata(ctx, "   ", "x"); err == nil {
			t.Fatal("SetAppMetadata(blank) error = nil, want error")
		} else if !strings.Contains(err.Error(), "app metadata key is required") {
			t.Fatalf("SetAppMetadata(blank) error = %v, want key-required message", err)
		}
		if _, _, err := db.GetAppMetadata(ctx, ""); err == nil {
			t.Fatal("GetAppMetadata(blank) error = nil, want error")
		} else if !strings.Contains(err.Error(), "app metadata key is required") {
			t.Fatalf("GetAppMetadata(blank) error = %v, want key-required message", err)
		}
	})

	t.Run("Should delete a key", func(t *testing.T) {
		if err := db.DeleteAppMetadata(ctx, "onboarding.completed"); err != nil {
			t.Fatalf("DeleteAppMetadata() error = %v", err)
		}
		_, found, err := db.GetAppMetadata(ctx, "onboarding.completed")
		if err != nil {
			t.Fatalf("GetAppMetadata() error = %v", err)
		}
		if found {
			t.Fatalf("GetAppMetadata() found = true after delete, want false")
		}
	})
}

func TestAppMetadataReopenAfterRestart(t *testing.T) {
	t.Parallel()

	t.Run("Should persist app metadata after reopening the database", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, GlobalDatabaseName)
		ctx := testutil.Context(t)

		first, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		if err := first.SetAppMetadata(ctx, "onboarding.completed_at", "2026-05-25T00:00:00Z"); err != nil {
			t.Fatalf("SetAppMetadata() error = %v", err)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		second, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB() reopen error = %v", err)
		}
		t.Cleanup(func() {
			if err := second.Close(ctx); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})

		value, found, err := second.GetAppMetadata(ctx, "onboarding.completed_at")
		if err != nil {
			t.Fatalf("GetAppMetadata() error = %v", err)
		}
		if !found || value != "2026-05-25T00:00:00Z" {
			t.Fatalf("GetAppMetadata() = (%q, %v) after restart, want persisted value", value, found)
		}
	})
}

func TestOnboardingMetadataLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should report not completed on fresh database", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)

		status, err := db.GetOnboardingStatus(ctx)
		if err != nil {
			t.Fatalf("GetOnboardingStatus() error = %v", err)
		}
		if status.Completed || status.CompletedAt != "" {
			t.Fatalf("GetOnboardingStatus() = %#v, want not completed", status)
		}
	})

	t.Run("Should complete once and preserve the first timestamp", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)

		first, err := db.CompleteOnboarding(ctx, "2026-05-25T12:00:00Z")
		if err != nil {
			t.Fatalf("CompleteOnboarding(first) error = %v", err)
		}
		if !first.Completed || first.CompletedAt != "2026-05-25T12:00:00Z" {
			t.Fatalf("CompleteOnboarding(first) = %#v, want first timestamp", first)
		}

		second, err := db.CompleteOnboarding(ctx, "2026-05-25T13:00:00Z")
		if err != nil {
			t.Fatalf("CompleteOnboarding(second) error = %v", err)
		}
		if !second.Completed || second.CompletedAt != first.CompletedAt {
			t.Fatalf("CompleteOnboarding(second) = %#v, want preserved %q", second, first.CompletedAt)
		}
	})

	t.Run("Should reset completed status", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)

		if _, err := db.CompleteOnboarding(ctx, "2026-05-25T12:00:00Z"); err != nil {
			t.Fatalf("CompleteOnboarding() error = %v", err)
		}
		status, err := db.ResetOnboarding(ctx)
		if err != nil {
			t.Fatalf("ResetOnboarding() error = %v", err)
		}
		if status.Completed || status.CompletedAt != "" {
			t.Fatalf("ResetOnboarding() = %#v, want not completed", status)
		}
	})

	t.Run("Should reject blank completion timestamp", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := testutil.Context(t)

		if _, err := db.CompleteOnboarding(ctx, "   "); err == nil {
			t.Fatal("CompleteOnboarding(blank) error = nil, want error")
		} else if !strings.Contains(err.Error(), "onboarding completed_at is required") {
			t.Fatalf("CompleteOnboarding(blank) error = %v, want completed_at-required message", err)
		}
	})
}
