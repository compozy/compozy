package globaldb

import (
	"database/sql"
	"errors"
	"slices"
	"testing"
	"time"

	presetspkg "github.com/compozy/agh/internal/notifications/presets"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBNotificationPresetSchema(t *testing.T) {
	t.Parallel()

	t.Run("Should create schema and seed disabled built-ins on fresh DB", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)

		assertNotificationPresetSchema(t, globalDB.db)
		items, err := globalDB.ListPresets(
			ctx,
			presetspkg.Query{BuiltIn: boolPtrForNotificationPresetTest(true)},
		)
		if err != nil {
			t.Fatalf("ListPresets(built-in) error = %v", err)
		}
		assertSeededNotificationPresetDefaults(t, items)
	})

	t.Run("Should map duplicate preset names from a real SQLite constraint", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		preset := presetspkg.Preset{
			Name:    "operator-terminal",
			Events:  []string{"task.run_failed"},
			Enabled: true,
		}
		if _, err := globalDB.CreatePreset(ctx, preset); err != nil {
			t.Fatalf("CreatePreset(first) error = %v", err)
		}
		if _, err := globalDB.CreatePreset(ctx, preset); !errors.Is(err, presetspkg.ErrPresetDuplicateName) {
			t.Fatalf("CreatePreset(duplicate) error = %v, want ErrPresetDuplicateName", err)
		}
	})
}

func TestGlobalDBNotificationPresetDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve user-modified built-ins and flag default drift", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		enabled := true
		events := []string{"task.run_failed"}
		updated, err := globalDB.UpdatePreset(
			ctx,
			presetspkg.BuiltInTaskTerminal,
			presetspkg.UpdateRequest{
				Events:  &events,
				Enabled: &enabled,
				Now:     time.Date(2026, 5, 21, 13, 0, 0, 0, time.UTC),
			},
		)
		if err != nil {
			t.Fatalf("UpdatePreset(built-in) error = %v", err)
		}
		if !updated.UserModified {
			t.Fatalf(
				"updated.UserModified = false, want built-in drift after operator edit: %#v",
				updated,
			)
		}

		defaults := presetspkg.BuiltInPresets(time.Date(2026, 5, 22, 9, 0, 0, 0, time.UTC))
		for index := range defaults {
			if defaults[index].Name == presetspkg.BuiltInTaskTerminal {
				defaults[index].Events = []string{"task.run_*", "task.run_review_*"}
				defaults[index].DefaultVersion = "2"
				defaults[index].DefaultHash = presetspkg.MutableHash(defaults[index])
			}
		}
		if err := globalDB.EnsureBuiltInPresets(ctx, defaults); err != nil {
			t.Fatalf("EnsureBuiltInPresets(updated defaults) error = %v", err)
		}

		stored, err := globalDB.GetPreset(ctx, presetspkg.BuiltInTaskTerminal)
		if err != nil {
			t.Fatalf("GetPreset(task_terminal) error = %v", err)
		}
		if !stored.Enabled || !slices.Equal(stored.Events, []string{"task.run_failed"}) {
			t.Fatalf(
				"stored mutable fields = enabled %t events %#v, want operator edits preserved",
				stored.Enabled,
				stored.Events,
			)
		}
		if !stored.UserModified || !stored.DefaultUpdateAvailable || stored.DefaultVersion != "2" {
			t.Fatalf(
				"stored default drift = %#v, want modified with update available at v2",
				stored,
			)
		}
	})
}

func assertNotificationPresetSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	assertTableColumns(t, db, "notification_presets", []string{
		"name",
		"events",
		"targets",
		"filter",
		"enabled",
		"built_in",
		"default_version",
		"default_hash",
		"user_modified",
		"default_update_available",
		"created_at",
		"updated_at",
	})
}

func assertSeededNotificationPresetDefaults(t *testing.T, items []presetspkg.Preset) {
	t.Helper()
	if got, want := len(items), 3; got != want {
		t.Fatalf("len(seed presets) = %d, want %d: %#v", got, want, items)
	}
	wantNames := []string{
		presetspkg.BuiltInProviderFailure,
		presetspkg.BuiltInSessionUnhealthy,
		presetspkg.BuiltInTaskTerminal,
	}
	gotNames := make([]string, 0, len(items))
	for _, item := range items {
		gotNames = append(gotNames, item.Name)
		if item.Enabled || !item.BuiltIn || item.DefaultVersion == "" || item.DefaultHash == "" ||
			item.UserModified || item.DefaultUpdateAvailable {
			t.Fatalf(
				"seed preset %q = %#v, want disabled built-in default metadata",
				item.Name,
				item,
			)
		}
	}
	slices.Sort(gotNames)
	if !slices.Equal(gotNames, wantNames) {
		t.Fatalf("seed names = %#v, want %#v", gotNames, wantNames)
	}
}

func boolPtrForNotificationPresetTest(value bool) *bool {
	copyValue := value
	return &copyValue
}
