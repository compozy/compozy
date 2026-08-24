package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

func TestNotificationPresetCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should list presets through daemon client filters", func(t *testing.T) {
		t.Parallel()

		var captured NotificationPresetQuery
		deps := newDefaultProfileTestDeps(t, &stubClient{
			listNotificationPresetsFn: func(
				_ context.Context,
				query NotificationPresetQuery,
			) (NotificationPresetListRecord, error) {
				captured = query
				return NotificationPresetListRecord{
					Presets: []NotificationPresetRecord{notificationPresetRecordForTest("task_terminal")},
					Total:   1,
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"notification-preset", "list", "--enabled", "--built-in", "--name", "task_terminal", "-o", "json",
		)
		if err != nil {
			t.Fatalf("notifications presets list error = %v", err)
		}
		if captured.Enabled == nil || !*captured.Enabled || captured.BuiltIn == nil || !*captured.BuiltIn ||
			captured.Name != "task_terminal" {
			t.Fatalf("captured query = %#v, want enabled built-in task_terminal", captured)
		}
		var payload NotificationPresetListRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(list stdout) error = %v\nstdout=%s", err, stdout)
		}
		if payload.Total != 1 || payload.Presets[0].Name != "task_terminal" {
			t.Fatalf("payload = %#v, want task_terminal list", payload)
		}
	})

	t.Run("Should create preset with event and target payload", func(t *testing.T) {
		t.Parallel()

		var captured CreateNotificationPresetRequest
		deps := newDefaultProfileTestDeps(t, &stubClient{
			createNotificationPresetFn: func(
				_ context.Context,
				request CreateNotificationPresetRequest,
			) (NotificationPresetRecord, error) {
				captured = request
				record := notificationPresetRecordForTest(request.Name)
				record.Events = request.Events
				record.Targets = request.Targets
				record.Filter = request.Filter
				record.Enabled = true
				return record, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"notification-preset", "create", "provider_failure_copy",
			"--event", "provider.*",
			"--target", " brg-1 : #ops ",
			"--filter", "severity >= warning",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("notifications preset create error = %v", err)
		}
		if captured.Name != "provider_failure_copy" || len(captured.Events) != 1 ||
			len(captured.Targets) != 1 || captured.Targets[0].BridgeID != " brg-1 " ||
			captured.Targets[0].CanonicalRoute != " #ops " || captured.Filter != "severity >= warning" {
			t.Fatalf("captured request = %#v", captured)
		}
		var payload NotificationPresetRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(create stdout) error = %v\nstdout=%s", err, stdout)
		}
		if payload.Name != "provider_failure_copy" || !payload.Enabled {
			t.Fatalf("payload = %#v, want enabled provider_failure_copy", payload)
		}
	})

	t.Run("Should update preset with mutable fields", func(t *testing.T) {
		t.Parallel()

		var capturedName string
		var captured UpdateNotificationPresetRequest
		deps := newDefaultProfileTestDeps(t, &stubClient{
			updateNotificationPresetFn: func(
				_ context.Context,
				name string,
				request UpdateNotificationPresetRequest,
			) (NotificationPresetRecord, error) {
				capturedName = name
				captured = request
				record := notificationPresetRecordForTest(name)
				if request.Events != nil {
					record.Events = *request.Events
				}
				if request.Targets != nil {
					record.Targets = *request.Targets
				}
				if request.Filter != nil {
					record.Filter = *request.Filter
				}
				record.Enabled = true
				return record, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"notification-preset", "update", "task_terminal",
			"--event", "task.run_failed",
			"--target", "brg-1:slack:channel:ops",
			"--filter", "outcome = failure",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("notifications preset update error = %v", err)
		}
		if capturedName != "task_terminal" ||
			captured.Events == nil ||
			len(*captured.Events) != 1 ||
			(*captured.Events)[0] != "task.run_failed" ||
			captured.Targets == nil ||
			len(*captured.Targets) != 1 ||
			(*captured.Targets)[0].BridgeID != "brg-1" ||
			(*captured.Targets)[0].CanonicalRoute != "slack:channel:ops" ||
			captured.Filter == nil ||
			*captured.Filter != "outcome = failure" {
			t.Fatalf("captured update = name %q request %#v", capturedName, captured)
		}
		var payload NotificationPresetRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(update stdout) error = %v\nstdout=%s", err, stdout)
		}
		if payload.Name != "task_terminal" || !payload.Enabled || payload.Filter != "outcome = failure" {
			t.Fatalf("payload = %#v, want enabled filtered task_terminal", payload)
		}
	})

	t.Run("Should reject update without changed fields and removed enable flags", func(t *testing.T) {
		t.Parallel()

		deps := newDefaultProfileTestDeps(t, &stubClient{})
		if _, _, err := executeRootCommand(
			t,
			deps,
			"notification-preset", "update", "task_terminal",
		); err == nil {
			t.Fatal("notifications preset update without flags error = nil, want error")
		} else if !strings.Contains(err.Error(), "at least one update flag is required") {
			t.Fatalf("notifications preset update without flags error = %v, want no-op validation", err)
		}
		if _, _, err := executeRootCommand(
			t,
			deps,
			"notification-preset", "update", "task_terminal",
			"--enabled",
		); err == nil {
			t.Fatal("notifications preset update --enabled error = nil, want removed flag error")
		} else if !strings.Contains(err.Error(), "unknown flag") {
			t.Fatalf("notifications preset update --enabled error = %v, want removed flag", err)
		}
	})

	t.Run("Should enable preset for the resolved profile", func(t *testing.T) {
		t.Parallel()

		var capturedName string
		var capturedProfile string
		var capturedEnabled bool
		deps := newDefaultProfileTestDeps(t, &stubClient{
			listProfilesFn: func(context.Context) ([]contract.Profile, error) {
				return []contract.Profile{{
					ID: "00000000000000000000000000", Name: "default", State: "active",
				}}, nil
			},
			listProfileSelectionsFn: func(context.Context) ([]contract.ProfileSelection, error) {
				return nil, nil
			},
			setNotificationPresetEnablementFn: func(
				_ context.Context,
				name string,
				request contract.SetNotificationPresetEnablementRequest,
			) (contract.NotificationPresetEnablementPayload, error) {
				capturedName = name
				capturedProfile = request.Profile
				capturedEnabled = request.Enabled
				return contract.NotificationPresetEnablementPayload{
					Name: name, Profile: request.Profile, Enabled: request.Enabled,
				}, nil
			},
		})

		if _, _, err := executeRootCommand(
			t,
			deps,
			"notification-preset", "enable", "task_terminal", "-o", "json",
		); err != nil {
			t.Fatalf("notifications preset enable error = %v", err)
		}
		if capturedName != "task_terminal" || capturedProfile != "default" || !capturedEnabled {
			t.Fatalf(
				"captured enablement = name %q profile %q enabled %t",
				capturedName,
				capturedProfile,
				capturedEnabled,
			)
		}
	})
}

func notificationPresetRecordForTest(name string) NotificationPresetRecord {
	return NotificationPresetRecord{
		Name:           name,
		Events:         []string{"task.run_*"},
		Targets:        []NotificationPresetTarget{{BridgeID: "brg-1", CanonicalRoute: "#ops"}},
		Enabled:        false,
		BuiltIn:        true,
		DefaultVersion: "1",
		DefaultHash:    "sha256:default",
		CreatedAt:      time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC),
	}
}
