package toolmeta_test

import (
	"testing"

	"github.com/compozy/compozy/internal/toolmeta"
	"github.com/compozy/compozy/internal/tools/builtin"
)

func TestNativeEntryMatchesBuiltinDescriptorInventory(t *testing.T) {
	t.Parallel()

	wantEntries := expectedNativeEntries()
	registeredEntries := toolmeta.NativeEntriesForTest()
	descriptorIDs := make(map[string]struct{}, len(wantEntries))
	for _, descriptor := range builtin.NativeDescriptors() {
		toolID := descriptor.ID.String()
		if _, duplicate := descriptorIDs[toolID]; duplicate {
			t.Fatalf("builtin descriptor inventory contains duplicate %q", toolID)
		}
		descriptorIDs[toolID] = struct{}{}
	}

	if len(descriptorIDs) != len(wantEntries) || len(registeredEntries) != len(wantEntries) {
		t.Fatalf(
			"native inventory sizes: builtin=%d registry=%d expectations=%d",
			len(descriptorIDs),
			len(registeredEntries),
			len(wantEntries),
		)
	}

	for toolID, wantEntry := range wantEntries {
		t.Run("Should expose exact metadata for "+toolID, func(t *testing.T) {
			t.Parallel()

			if _, exists := descriptorIDs[toolID]; !exists {
				t.Fatalf("builtin descriptor inventory is missing %q", toolID)
			}
			registeredEntry, exists := registeredEntries[toolID]
			if !exists {
				t.Fatalf("presentation registry is missing %q", toolID)
			}
			if registeredEntry != wantEntry {
				t.Fatalf("registered metadata for %q = %#v, want %#v", toolID, registeredEntry, wantEntry)
			}
			gotEntry, ok := toolmeta.NativeEntry(toolID)
			if !ok {
				t.Fatalf("NativeEntry(%q) ok = false", toolID)
			}
			if gotEntry != wantEntry {
				t.Fatalf("NativeEntry(%q) = %#v, want %#v", toolID, gotEntry, wantEntry)
			}
		})
	}

	for toolID := range descriptorIDs {
		if _, exists := wantEntries[toolID]; !exists {
			t.Fatalf("builtin descriptor %q has no exact metadata expectation", toolID)
		}
	}
	for toolID := range registeredEntries {
		if _, exists := descriptorIDs[toolID]; !exists {
			t.Fatalf("presentation registry contains stale native ID %q", toolID)
		}
	}

	t.Run("Should reject IDs outside the exact native inventory", func(t *testing.T) {
		t.Parallel()

		for _, toolID := range []string{"custom__task_create", "compozy__task_future", "compozy__"} {
			if _, ok := toolmeta.NativeEntry(toolID); ok {
				t.Fatalf("NativeEntry(%q) ok = true, want external fallback", toolID)
			}
		}
	})
}

func expectedNativeEntries() map[string]toolmeta.Entry {
	return map[string]toolmeta.Entry{
		"compozy__agent_create":                   expectedNativeEntry("Creating", " ", false, "🤖", "auto"),
		"compozy__agent_heartbeat_status":         expectedNativeEntry("Reading", " ", false, "🤖", "auto"),
		"compozy__agent_heartbeat_wake":           expectedNativeEntry("Running", " ", false, "🤖", "auto"),
		"compozy__automation_jobs_create":         expectedNativeEntry("Creating", " ", false, "⏱️", "auto"),
		"compozy__automation_jobs_delete":         expectedNativeEntry("Deleting", " ", false, "⏱️", "auto"),
		"compozy__automation_jobs_disable":        expectedNativeEntry("Disabling", " ", false, "⏱️", "auto"),
		"compozy__automation_jobs_enable":         expectedNativeEntry("Enabling", " ", false, "⏱️", "auto"),
		"compozy__automation_jobs_get":            expectedNativeEntry("Reading", " ", false, "⏱️", "auto"),
		"compozy__automation_jobs_history":        expectedNativeEntry("Reading", " ", false, "⏱️", "auto"),
		"compozy__automation_jobs_list":           expectedNativeEntry("Reading", " ", false, "⏱️", "auto"),
		"compozy__automation_jobs_trigger":        expectedNativeEntry("Running", " ", false, "⏱️", "auto"),
		"compozy__automation_jobs_update":         expectedNativeEntry("Updating", " ", false, "⏱️", "auto"),
		"compozy__automation_runs_get":            expectedNativeEntry("Reading", " ", false, "⏱️", "auto"),
		"compozy__automation_runs_list":           expectedNativeEntry("Reading", " ", false, "⏱️", "auto"),
		"compozy__automation_suggestions_accept":  expectedNativeEntry("Creating", " ", false, "⏱️", "auto"),
		"compozy__automation_suggestions_dismiss": expectedNativeEntry("Dismissing", " ", false, "⏱️", "auto"),
		"compozy__automation_suggestions_list":    expectedNativeEntry("Reading", " ", false, "⏱️", "auto"),
		"compozy__automation_triggers_create":     expectedNativeEntry("Creating", " ", false, "⏱️", "auto"),
		"compozy__automation_triggers_delete":     expectedNativeEntry("Deleting", " ", false, "⏱️", "auto"),
		"compozy__automation_triggers_disable":    expectedNativeEntry("Disabling", " ", false, "⏱️", "auto"),
		"compozy__automation_triggers_enable":     expectedNativeEntry("Enabling", " ", false, "⏱️", "auto"),
		"compozy__automation_triggers_get":        expectedNativeEntry("Reading", " ", false, "⏱️", "auto"),
		"compozy__automation_triggers_history":    expectedNativeEntry("Reading", " ", false, "⏱️", "auto"),
		"compozy__automation_triggers_list":       expectedNativeEntry("Reading", " ", false, "⏱️", "auto"),
		"compozy__automation_triggers_update":     expectedNativeEntry("Updating", " ", false, "⏱️", "auto"),
		"compozy__bridges_list":                   expectedNativeEntry("Reading", " ", false, "🌉", "auto"),
		"compozy__bridges_status":                 expectedNativeEntry("Reading", " ", false, "🌉", "auto"),
		"compozy__bundles_activate":               expectedNativeEntry("Activating", " ", false, "📦", "auto"),
		"compozy__bundles_deactivate":             expectedNativeEntry("Deactivating", " ", false, "📦", "auto"),
		"compozy__bundles_info":                   expectedNativeEntry("Reading", " ", false, "📦", "auto"),
		"compozy__bundles_list":                   expectedNativeEntry("Reading", " ", false, "📦", "auto"),
		"compozy__bundles_status":                 expectedNativeEntry("Reading", " ", false, "📦", "auto"),
		"compozy__clarify":                        expectedNativeEntry("Asking", " ", false, "💬", "auto"),
		"compozy__config_diff":                    expectedNativeEntry("Reading", " ", false, "⚙️", "auto"),
		"compozy__config_get":                     expectedNativeEntry("Reading", " ", false, "⚙️", "auto"),
		"compozy__config_list":                    expectedNativeEntry("Reading", " ", false, "⚙️", "auto"),
		"compozy__config_path":                    expectedNativeEntry("Reading", " ", false, "⚙️", "auto"),
		"compozy__config_set":                     expectedNativeEntry("Updating", " ", false, "⚙️", "auto"),
		"compozy__config_show":                    expectedNativeEntry("Reading", " ", true, "⚙️", "none"),
		"compozy__config_unset":                   expectedNativeEntry("Removing", " ", false, "⚙️", "auto"),
		"compozy__desktop_clients":                expectedNativeEntry("Reading", " ", false, "🪟", "auto"),
		"compozy__desktop_create":                 expectedNativeEntry("Creating", " ", false, "🪟", "auto"),
		"compozy__desktop_delete":                 expectedNativeEntry("Deleting", " ", false, "🪟", "auto"),
		"compozy__desktop_list":                   expectedNativeEntry("Reading", " ", false, "🪟", "auto"),
		"compozy__desktop_reorder":                expectedNativeEntry("Reordering", " ", false, "🪟", "auto"),
		"compozy__desktop_switch":                 expectedNativeEntry("Switching", " ", false, "🪟", "auto"),
		"compozy__desktop_update":                 expectedNativeEntry("Updating", " ", false, "🪟", "auto"),
		"compozy__extensions_disable":             expectedNativeEntry("Disabling", " ", false, "🧩", "auto"),
		"compozy__extensions_enable":              expectedNativeEntry("Enabling", " ", false, "🧩", "auto"),
		"compozy__extensions_info":                expectedNativeEntry("Reading", " ", false, "🧩", "auto"),
		"compozy__extensions_install":             expectedNativeEntry("Installing", " ", false, "🧩", "auto"),
		"compozy__extensions_list":                expectedNativeEntry("Reading", " ", false, "🧩", "auto"),
		"compozy__extensions_remove":              expectedNativeEntry("Removing", " ", false, "🧩", "auto"),
		"compozy__extensions_update":              expectedNativeEntry("Updating", " ", false, "🧩", "auto"),
		"compozy__goal_get":                       expectedNativeEntry("Reading", " ", false, "🎯", "auto"),
		"compozy__goal_report":                    expectedNativeEntry("Reporting", " ", false, "🎯", "auto"),
		"compozy__hooks_create":                   expectedNativeEntry("Creating", " ", false, "🪝", "auto"),
		"compozy__hooks_delete":                   expectedNativeEntry("Deleting", " ", false, "🪝", "auto"),
		"compozy__hooks_disable":                  expectedNativeEntry("Disabling", " ", false, "🪝", "auto"),
		"compozy__hooks_enable":                   expectedNativeEntry("Enabling", " ", false, "🪝", "auto"),
		"compozy__hooks_events":                   expectedNativeEntry("Reading", " ", false, "🪝", "auto"),
		"compozy__hooks_info":                     expectedNativeEntry("Reading", " ", false, "🪝", "auto"),
		"compozy__hooks_list":                     expectedNativeEntry("Reading", " ", false, "🪝", "auto"),
		"compozy__hooks_runs":                     expectedNativeEntry("Reading", " ", false, "🪝", "auto"),
		"compozy__hooks_update":                   expectedNativeEntry("Updating", " ", false, "🪝", "auto"),
		"compozy__layout_apply":                   expectedNativeEntry("Applying", " ", false, "🪟", "auto"),
		"compozy__layout_arrange":                 expectedNativeEntry("Arranging", " ", false, "🪟", "auto"),
		"compozy__layout_balance":                 expectedNativeEntry("Balancing", " ", false, "🪟", "auto"),
		"compozy__layout_export":                  expectedNativeEntry("Exporting", " ", false, "🪟", "auto"),
		"compozy__layout_get":                     expectedNativeEntry("Reading", " ", false, "🪟", "auto"),
		"compozy__layout_preview":                 expectedNativeEntry("Previewing", " ", false, "🪟", "auto"),
		"compozy__layout_redo":                    expectedNativeEntry("Redoing", " ", false, "🪟", "auto"),
		"compozy__layout_resize":                  expectedNativeEntry("Resizing", " ", false, "🪟", "auto"),
		"compozy__layout_undo":                    expectedNativeEntry("Undoing", " ", false, "🪟", "auto"),
		"compozy__layout_validate":                expectedNativeEntry("Validating", " ", false, "🪟", "auto"),
		"compozy__logs":                           expectedNativeEntry("Reading", " ", false, "📜", "query"),
		"compozy__marketplace_search":             expectedNativeEntry("Searching", " for ", false, "🧩", "query"),
		"compozy__loop_approve":                   expectedNativeEntry("Approving", " ", false, "🔁", "auto"),
		"compozy__loop_configure":                 expectedNativeEntry("Updating", " ", false, "🔁", "auto"),
		"compozy__loop_create":                    expectedNativeEntry("Creating", " ", false, "🔁", "auto"),
		"compozy__loop_delete":                    expectedNativeEntry("Deleting", " ", false, "🔁", "auto"),
		"compozy__loop_inspect":                   expectedNativeEntry("Reading", " ", false, "🔁", "auto"),
		"compozy__loop_list":                      expectedNativeEntry("Reading", " ", false, "🔁", "auto"),
		"compozy__loop_pause":                     expectedNativeEntry("Pausing", " ", false, "🔁", "auto"),
		"compozy__loop_resume":                    expectedNativeEntry("Resuming", " ", false, "🔁", "auto"),
		"compozy__loop_run":                       expectedNativeEntry("Running", " ", false, "🔁", "auto"),
		"compozy__loop_runs":                      expectedNativeEntry("Reading", " ", false, "🔁", "auto"),
		"compozy__loop_status":                    expectedNativeEntry("Reading", " ", false, "🔁", "auto"),
		"compozy__loop_stop":                      expectedNativeEntry("Stopping", " ", false, "🔁", "auto"),
		"compozy__loop_turns":                     expectedNativeEntry("Reading", " ", false, "🔁", "auto"),
		"compozy__loop_validate":                  expectedNativeEntry("Validating", " ", false, "🔁", "auto"),
		"compozy__mcp_auth_status":                expectedNativeEntry("Reading", " ", true, "🔌", "none"),
		"compozy__mcp_status":                     expectedNativeEntry("Reading", " ", false, "🔌", "auto"),
		"compozy__memory_admin_history":           expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_daily_list":              expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_decisions_list":          expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_decisions_revert":        expectedNativeEntry("Reverting", " ", false, "🧠", "auto"),
		"compozy__memory_decisions_show":          expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_dream_list":              expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_dream_retry":             expectedNativeEntry("Retrying", " ", false, "🧠", "auto"),
		"compozy__memory_dream_show":              expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_dream_status":            expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_dream_trigger":           expectedNativeEntry("Running", " ", false, "🧠", "auto"),
		"compozy__memory_extractor_drain":         expectedNativeEntry("Running", " ", false, "🧠", "auto"),
		"compozy__memory_extractor_failures":      expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_extractor_retry":         expectedNativeEntry("Retrying", " ", false, "🧠", "auto"),
		"compozy__memory_extractor_status":        expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_health":                  expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_list":                    expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_note":                    expectedNativeEntry("Saving", " ", false, "🧠", "auto"),
		"compozy__memory_promote":                 expectedNativeEntry("Promoting", " ", false, "🧠", "auto"),
		"compozy__memory_propose":                 expectedNativeEntry("Proposing", " ", false, "🧠", "auto"),
		"compozy__memory_provider_disable":        expectedNativeEntry("Disabling", " ", false, "🧠", "auto"),
		"compozy__memory_provider_enable":         expectedNativeEntry("Enabling", " ", false, "🧠", "auto"),
		"compozy__memory_provider_get":            expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_provider_list":           expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_provider_select":         expectedNativeEntry("Selecting", " ", false, "🧠", "auto"),
		"compozy__memory_recall_trace":            expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_reindex":                 expectedNativeEntry("Reindexing", " ", false, "🧠", "auto"),
		"compozy__memory_reload":                  expectedNativeEntry("Reloading", " ", false, "🧠", "auto"),
		"compozy__memory_reset":                   expectedNativeEntry("Resetting", " ", false, "🧠", "auto"),
		"compozy__memory_scope_show":              expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_search":                  expectedNativeEntry("Searching", " for ", false, "🧠", "query"),
		"compozy__memory_session_ledger":          expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__memory_session_replay":          expectedNativeEntry("Replaying", " ", false, "🧠", "auto"),
		"compozy__memory_sessions_prune":          expectedNativeEntry("Pruning", " ", false, "🧠", "auto"),
		"compozy__memory_sessions_repair":         expectedNativeEntry("Repairing", " ", false, "🧠", "auto"),
		"compozy__memory_show":                    expectedNativeEntry("Reading", " ", false, "🧠", "auto"),
		"compozy__network_channel_create":         expectedNativeEntry("Creating", " ", false, "🌐", "auto"),
		"compozy__network_channel_update":         expectedNativeEntry("Updating", " ", false, "🌐", "auto"),
		"compozy__network_channels":               expectedNativeEntry("Reading", " ", false, "🌐", "auto"),
		"compozy__network_direct_messages":        expectedNativeEntry("Reading", " ", false, "🌐", "auto"),
		"compozy__network_direct_resolve":         expectedNativeEntry("Resolving", " ", false, "🌐", "auto"),
		"compozy__network_directs":                expectedNativeEntry("Reading", " ", false, "🌐", "auto"),
		"compozy__network_inbox":                  expectedNativeEntry("Reading", " ", false, "🌐", "auto"),
		"compozy__network_mute":                   expectedNativeEntry("Muting", " ", false, "🌐", "auto"),
		"compozy__network_peers":                  expectedNativeEntry("Reading", " ", false, "🌐", "auto"),
		"compozy__network_send":                   expectedNativeEntry("Sending", " ", false, "🌐", "auto"),
		"compozy__network_status":                 expectedNativeEntry("Reading", " ", false, "🌐", "auto"),
		"compozy__network_subscribe":              expectedNativeEntry("Subscribing", " ", false, "🌐", "auto"),
		"compozy__network_subscriptions":          expectedNativeEntry("Reading", " ", false, "🌐", "auto"),
		"compozy__network_thread_messages":        expectedNativeEntry("Reading", " ", false, "🌐", "auto"),
		"compozy__network_threads":                expectedNativeEntry("Reading", " ", false, "🌐", "auto"),
		"compozy__network_unmute":                 expectedNativeEntry("Unmuting", " ", false, "🌐", "auto"),
		"compozy__network_usage":                  expectedNativeEntry("Reading", " ", false, "🌐", "auto"),
		"compozy__network_work":                   expectedNativeEntry("Reading", " ", false, "🌐", "auto"),
		"compozy__observe_metrics":                expectedNativeEntry("Reading", " ", false, "🔭", "auto"),
		"compozy__observe_search":                 expectedNativeEntry("Searching", " for ", false, "🔭", "query"),
		"compozy__provider_models_curate":         expectedNativeEntry("Curating", " ", false, "🤖", "auto"),
		"compozy__provider_models_list":           expectedNativeEntry("Reading", " ", false, "🤖", "auto"),
		"compozy__provider_models_refresh":        expectedNativeEntry("Refreshing", " ", false, "🤖", "auto"),
		"compozy__provider_models_status":         expectedNativeEntry("Reading", " ", false, "🤖", "auto"),
		"compozy__resources_info":                 expectedNativeEntry("Reading", " ", false, "📚", "auto"),
		"compozy__resources_list":                 expectedNativeEntry("Reading", " ", false, "📚", "auto"),
		"compozy__resources_snapshot":             expectedNativeEntry("Reading", " ", false, "📚", "auto"),
		"compozy__session_describe":               expectedNativeEntry("Reading", " ", false, "💬", "auto"),
		"compozy__session_events":                 expectedNativeEntry("Reading", " ", false, "💬", "auto"),
		"compozy__session_health":                 expectedNativeEntry("Reading", " ", false, "💬", "auto"),
		"compozy__session_history":                expectedNativeEntry("Reading", " ", false, "💬", "auto"),
		"compozy__session_list":                   expectedNativeEntry("Reading", " ", false, "💬", "auto"),
		"compozy__session_status":                 expectedNativeEntry("Reading", " ", false, "💬", "auto"),
		"compozy__skill_list":                     expectedNativeEntry("Reading", " ", false, "🧰", "auto"),
		"compozy__skill_search":                   expectedNativeEntry("Searching", " for ", false, "🧰", "query"),
		"compozy__skill_view":                     expectedNativeEntry("Reading", " ", false, "🧰", "auto"),
		"compozy__task_block":                     expectedNativeEntry("Blocking", " ", false, "✅", "auto"),
		"compozy__task_blocks":                    expectedNativeEntry("Reading", " ", false, "✅", "auto"),
		"compozy__task_cancel":                    expectedNativeEntry("Canceling", " ", false, "✅", "auto"),
		"compozy__task_child_create":              expectedNativeEntry("Creating", " ", false, "✅", "auto"),
		"compozy__task_create":                    expectedNativeEntry("Creating", " ", false, "✅", "auto"),
		"compozy__task_execution_profile_delete":  expectedNativeEntry("Deleting", " ", false, "✅", "auto"),
		"compozy__task_execution_profile_get":     expectedNativeEntry("Reading", " ", false, "✅", "auto"),
		"compozy__task_execution_profile_set":     expectedNativeEntry("Updating", " ", false, "✅", "auto"),
		"compozy__task_fanout_runs":               expectedNativeEntry("Creating", " ", false, "✅", "auto"),
		"compozy__task_list":                      expectedNativeEntry("Reading", " ", false, "✅", "auto"),
		"compozy__task_notification_delete":       expectedNativeEntry("Deleting", " ", false, "✅", "auto"),
		"compozy__task_notification_list":         expectedNativeEntry("Reading", " ", false, "✅", "auto"),
		"compozy__task_notification_show":         expectedNativeEntry("Reading", " ", false, "✅", "auto"),
		"compozy__task_notification_subscribe":    expectedNativeEntry("Subscribing", " ", false, "✅", "auto"),
		"compozy__task_promote_from_thread":       expectedNativeEntry("Promoting", " ", false, "✅", "auto"),
		"compozy__task_read":                      expectedNativeEntry("Reading", " ", false, "✅", "auto"),
		"compozy__task_recover":                   expectedNativeEntry("Recovering", " ", false, "✅", "auto"),
		"compozy__task_run_claim_next":            expectedNativeEntry("Claiming", " ", false, "✅", "auto"),
		"compozy__task_run_complete":              expectedNativeEntry("Completing", " ", false, "✅", "auto"),
		"compozy__task_run_fail":                  expectedNativeEntry("Reporting failure", " ", false, "✅", "auto"),
		"compozy__task_run_heartbeat":             expectedNativeEntry("Renewing", " ", false, "✅", "auto"),
		"compozy__task_run_list":                  expectedNativeEntry("Reading", " ", false, "✅", "auto"),
		"compozy__task_run_release":               expectedNativeEntry("Releasing", " ", false, "✅", "auto"),
		"compozy__task_run_review_list":           expectedNativeEntry("Reading", " ", false, "✅", "auto"),
		"compozy__task_run_review_request":        expectedNativeEntry("Requesting review", " ", false, "✅", "auto"),
		"compozy__task_run_review_show":           expectedNativeEntry("Reading", " ", false, "✅", "auto"),
		"compozy__task_run_review_submit":         expectedNativeEntry("Submitting review", " ", false, "✅", "auto"),
		"compozy__task_unblock":                   expectedNativeEntry("Unblocking", " ", false, "✅", "auto"),
		"compozy__task_update":                    expectedNativeEntry("Updating", " ", false, "✅", "auto"),
		"compozy__tool_approvals_set":             expectedNativeEntry("Setting", " ", false, "🔧", "auto"),
		"compozy__tool_approvals_list":            expectedNativeEntry("Reading", " ", false, "🔧", "auto"),
		"compozy__tool_approvals_revoke":          expectedNativeEntry("Revoking", " ", false, "🔧", "auto"),
		"compozy__tool_artifact_read":             expectedNativeEntry("Reading", " ", false, "🔧", "auto"),
		"compozy__tool_info":                      expectedNativeEntry("Reading", " ", false, "🔧", "auto"),
		"compozy__tool_list":                      expectedNativeEntry("Reading", " ", false, "🔧", "auto"),
		"compozy__tool_search":                    expectedNativeEntry("Searching", " for ", false, "🔧", "query"),
		"compozy__window_close":                   expectedNativeEntry("Closing", " ", false, "🪟", "auto"),
		"compozy__window_float":                   expectedNativeEntry("Toggling", " ", false, "🪟", "auto"),
		"compozy__window_focus":                   expectedNativeEntry("Focusing", " ", false, "🪟", "auto"),
		"compozy__window_list":                    expectedNativeEntry("Reading", " ", false, "🪟", "auto"),
		"compozy__window_move":                    expectedNativeEntry("Moving", " ", false, "🪟", "auto"),
		"compozy__window_navigate":                expectedNativeEntry("Navigating", " ", false, "🪟", "auto"),
		"compozy__window_open":                    expectedNativeEntry("Opening", " ", false, "🪟", "auto"),
		"compozy__window_swap":                    expectedNativeEntry("Swapping", " ", false, "🪟", "auto"),
		"compozy__window_zoom":                    expectedNativeEntry("Zooming", " ", false, "🪟", "auto"),
		"compozy__workspace_describe":             expectedNativeEntry("Reading", " ", false, "🗂️", "auto"),
		"compozy__workspace_info":                 expectedNativeEntry("Reading", " ", false, "🗂️", "auto"),
		"compozy__workspace_list":                 expectedNativeEntry("Reading", " ", false, "🗂️", "auto"),
	}
}

func expectedNativeEntry(
	verb string,
	connector string,
	dropsPreview bool,
	emoji string,
	preview string,
) toolmeta.Entry {
	return toolmeta.Entry{
		Verb:         verb,
		Connector:    connector,
		DropsPreview: dropsPreview,
		Emoji:        emoji,
		Preview:      preview,
	}
}
