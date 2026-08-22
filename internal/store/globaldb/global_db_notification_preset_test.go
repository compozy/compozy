package globaldb

import (
	"database/sql"
	"errors"
	"slices"
	"testing"
	"time"

	presetspkg "github.com/compozy/compozy/internal/notifications/presets"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGlobalDBNotificationPresetSchema(t *testing.T) {
	t.Parallel()

	t.Run("Should create schema and project built-ins enabled by default", func(t *testing.T) {
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

	// Invariant: preset enablement is a disabled exception per profile; absence
	// means enabled and changing one profile cannot affect another.
	// Owner: global notification preset store.
	// Canonical suite: global DB notification preset tests.
	t.Run("Should isolate default-on preset enablement by profile", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		const financeID = "01JPROFILEFINANCE000000000"
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO profiles (
			id, name, color, icon, state, created_at
		) VALUES (?, 'finance', '#112233', 'briefcase', 'active', ?)`, financeID, phase0FixtureTime); err != nil {
			t.Fatalf("insert finance profile: %v", err)
		}
		if err := globalDB.SetPresetEnabled(ctx, presetspkg.BuiltInTaskTerminal, financeID, false); err != nil {
			t.Fatalf("SetPresetEnabled(disable finance) error = %v", err)
		}
		finance, err := globalDB.ListPresetsForProfile(
			ctx, presetspkg.Query{Name: presetspkg.BuiltInTaskTerminal}, financeID,
		)
		if err != nil || len(finance) != 1 || finance[0].Enabled {
			t.Fatalf("finance presets = %#v, %v, want one disabled", finance, err)
		}
		defaults, err := globalDB.ListPresetsForProfile(
			ctx, presetspkg.Query{Name: presetspkg.BuiltInTaskTerminal}, store.DefaultProfileID,
		)
		if err != nil || len(defaults) != 1 || !defaults[0].Enabled {
			t.Fatalf("default presets = %#v, %v, want one enabled", defaults, err)
		}
		if err := globalDB.SetPresetEnabled(ctx, presetspkg.BuiltInTaskTerminal, financeID, true); err != nil {
			t.Fatalf("SetPresetEnabled(enable finance) error = %v", err)
		}
		var exceptions int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_preset_enablement
			WHERE preset_name = ? AND profile_id = ?`, presetspkg.BuiltInTaskTerminal, financeID).Scan(&exceptions); err != nil {
			t.Fatalf("count preset exceptions: %v", err)
		}
		if exceptions != 0 {
			t.Fatalf("preset exception rows = %d, want 0", exceptions)
		}
	})
}

func TestGlobalDBNotificationPresetDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve user-modified built-ins and flag default drift", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		events := []string{"task.run_failed"}
		updated, err := globalDB.UpdatePreset(
			ctx,
			presetspkg.BuiltInTaskTerminal,
			presetspkg.UpdateRequest{
				Events: &events,
				Now:    time.Date(2026, 5, 21, 13, 0, 0, 0, time.UTC),
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
		if !item.Enabled || !item.BuiltIn || item.DefaultVersion == "" || item.DefaultHash == "" ||
			item.UserModified || item.DefaultUpdateAvailable {
			t.Fatalf(
				"seed preset %q = %#v, want default-on built-in metadata",
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
