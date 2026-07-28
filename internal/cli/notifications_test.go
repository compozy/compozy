package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNotificationPresetCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should list presets through daemon client filters", func(t *testing.T) {
		t.Parallel()

		var captured NotificationPresetQuery
		deps := newTestDeps(t, &stubClient{
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
			"notifications", "presets", "list", "--enabled", "--built-in", "--name", "task_terminal", "-o", "json",
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
		deps := newTestDeps(t, &stubClient{
			createNotificationPresetFn: func(
				_ context.Context,
				request CreateNotificationPresetRequest,
			) (NotificationPresetRecord, error) {
				captured = request
				record := notificationPresetRecordForTest(request.Name)
				record.Events = request.Events
				record.Targets = request.Targets
				record.Filter = request.Filter
				record.Enabled = request.Enabled
				return record, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"notifications", "preset", "create", "provider_failure_copy",
			"--event", "provider.*",
			"--target", "brg-1:#ops",
			"--filter", "severity >= warning",
			"--enabled",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("notifications preset create error = %v", err)
		}
		if captured.Name != "provider_failure_copy" || len(captured.Events) != 1 ||
			len(captured.Targets) != 1 || captured.Targets[0].BridgeID != "brg-1" ||
			captured.Targets[0].CanonicalRoute != "#ops" || captured.Filter != "severity >= warning" ||
			!captured.Enabled {
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
		deps := newTestDeps(t, &stubClient{
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
				record.Enabled = request.Enabled != nil && *request.Enabled
				return record, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"notifications", "preset", "update", "task_terminal",
			"--event", "task.run_failed",
			"--target", "brg-1:slack:channel:ops",
			"--filter", "outcome = failure",
			"--enabled",
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
			*captured.Filter != "outcome = failure" ||
			captured.Enabled == nil ||
			!*captured.Enabled {
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

	t.Run("Should reject update without changed fields or with conflicting enable flags", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		if _, _, err := executeRootCommand(
			t,
			deps,
			"notifications", "preset", "update", "task_terminal",
		); err == nil {
			t.Fatal("notifications preset update without flags error = nil, want error")
		} else if !strings.Contains(err.Error(), "at least one update flag is required") {
			t.Fatalf("notifications preset update without flags error = %v, want no-op validation", err)
		}
		if _, _, err := executeRootCommand(
			t,
			deps,
			"notifications", "preset", "update", "task_terminal",
			"--enabled",
			"--disabled",
		); err == nil {
			t.Fatal("notifications preset update --enabled --disabled error = nil, want error")
		} else if !strings.Contains(err.Error(), "use either --enabled or --disabled, not both") {
			t.Fatalf("notifications preset update --enabled --disabled error = %v, want conflict validation", err)
		}
	})

	t.Run("Should enable preset with replacement targets", func(t *testing.T) {
		t.Parallel()

		var capturedName string
		var captured UpdateNotificationPresetRequest
		deps := newTestDeps(t, &stubClient{
			updateNotificationPresetFn: func(
				_ context.Context,
				name string,
				request UpdateNotificationPresetRequest,
			) (NotificationPresetRecord, error) {
				capturedName = name
				captured = request
				record := notificationPresetRecordForTest(name)
				record.Enabled = request.Enabled != nil && *request.Enabled
				if request.Targets != nil {
					record.Targets = *request.Targets
				}
				return record, nil
			},
		})

		if _, _, err := executeRootCommand(
			t,
			deps,
			"notifications", "preset", "enable", "task_terminal", "--target", "brg-1:#ops", "-o", "json",
		); err != nil {
			t.Fatalf("notifications preset enable error = %v", err)
		}
		if capturedName != "task_terminal" || captured.Enabled == nil || !*captured.Enabled ||
			captured.Targets == nil || len(*captured.Targets) != 1 {
			t.Fatalf("captured update = name %q request %#v", capturedName, captured)
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
