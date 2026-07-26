package hooks

import (
	"encoding/json"
	"testing"

	"github.com/compozy/agh/internal/network/participation"
)

func TestCloneAsyncPayloadCopiesReferenceFields(t *testing.T) {
	t.Parallel()

	t.Run("Should sandbox prepare", func(t *testing.T) {
		t.Parallel()

		original := SandboxPreparePayload{
			Profile: SandboxProfilePayload{
				Env: map[string]string{"PATH": "/usr/bin"},
			},
			LocalAdditionalDirs: []string{"/tmp/a"},
			AgentEnv:            []string{"KEY=before"},
			EnvOverrides:        map[string]string{"KEY": "before"},
		}
		cloned := cloneAsyncPayload(original)

		original.Profile.Env["PATH"] = "/bin"
		original.LocalAdditionalDirs[0] = "/tmp/b"
		original.AgentEnv[0] = "KEY=after"
		original.EnvOverrides["KEY"] = "after"

		if cloned.Profile.Env["PATH"] != "/usr/bin" {
			t.Fatalf("cloned profile env = %q, want %q", cloned.Profile.Env["PATH"], "/usr/bin")
		}
		if cloned.LocalAdditionalDirs[0] != "/tmp/a" {
			t.Fatalf("cloned local dir = %q, want %q", cloned.LocalAdditionalDirs[0], "/tmp/a")
		}
		if cloned.AgentEnv[0] != "KEY=before" {
			t.Fatalf("cloned agent env = %q, want %q", cloned.AgentEnv[0], "KEY=before")
		}
		if cloned.EnvOverrides["KEY"] != "before" {
			t.Fatalf("cloned env override = %q, want %q", cloned.EnvOverrides["KEY"], "before")
		}
	})

	t.Run("Should sandbox lifecycle slices", func(t *testing.T) {
		t.Parallel()

		ready := cloneAsyncPayload(SandboxReadyPayload{RuntimeAdditionalDirs: []string{"/runtime"}})
		syncBefore := cloneAsyncPayload(SandboxSyncBeforePayload{ExcludePatterns: []string{"*.tmp"}})
		syncAfter := cloneAsyncPayload(SandboxSyncAfterPayload{Errors: []string{"before"}})

		if ready.RuntimeAdditionalDirs[0] != "/runtime" {
			t.Fatalf("cloned runtime dir = %q, want %q", ready.RuntimeAdditionalDirs[0], "/runtime")
		}
		if syncBefore.ExcludePatterns[0] != "*.tmp" {
			t.Fatalf("cloned exclude pattern = %q, want %q", syncBefore.ExcludePatterns[0], "*.tmp")
		}
		if syncAfter.Errors[0] != "before" {
			t.Fatalf("cloned error entry = %q, want %q", syncAfter.Errors[0], "before")
		}
	})

	t.Run("Should prompt and context payloads", func(t *testing.T) {
		t.Parallel()

		original := ContextCompactPayload{
			ContextBlocks: []ContextBlock{{
				Kind: "note",
				Text: "before",
				Metadata: map[string]string{
					"scope": "before",
				},
			}},
		}
		cloned := cloneAsyncPayload(original)

		original.ContextBlocks[0].Text = "after"
		original.ContextBlocks[0].Metadata["scope"] = "after"

		if cloned.ContextBlocks[0].Text != "before" {
			t.Fatalf("cloned context text = %q, want %q", cloned.ContextBlocks[0].Text, "before")
		}
		if cloned.ContextBlocks[0].Metadata["scope"] != "before" {
			t.Fatalf("cloned context metadata = %q, want %q", cloned.ContextBlocks[0].Metadata["scope"], "before")
		}

		prompt := PromptPayload{
			ContextBlocks: []ContextBlock{{
				Kind: "prompt",
				Text: "prompt-before",
			}},
		}
		promptClone := cloneAsyncPayload(prompt)
		prompt.ContextBlocks[0].Text = "prompt-after"
		if promptClone.ContextBlocks[0].Text != "prompt-before" {
			t.Fatalf("cloned prompt text = %q, want %q", promptClone.ContextBlocks[0].Text, "prompt-before")
		}
	})

	t.Run("Should event and message raw json", func(t *testing.T) {
		t.Parallel()

		event := EventRecordPayload{Content: json.RawMessage(`{"value":"before"}`)}
		eventClone := cloneAsyncPayload(event)
		event.Content[10] = 'X'
		if string(eventClone.Content) != `{"value":"before"}` {
			t.Fatalf("cloned event content = %s, want original payload", string(eventClone.Content))
		}

		message := MessagePayload{Raw: json.RawMessage(`{"message":"before"}`)}
		messageClone := cloneAsyncPayload(message)
		message.Raw[12] = 'X'
		if string(messageClone.Raw) != `{"message":"before"}` {
			t.Fatalf("cloned message raw = %s, want original payload", string(messageClone.Raw))
		}
	})

	t.Run("Should automation payloads", func(t *testing.T) {
		t.Parallel()

		job := AutomationJobPreFirePayload{
			Schedule: &AutomationSchedulePayload{Mode: "cron", Expr: "0 * * * *"},
		}
		jobClone := cloneAsyncPayload(job)
		job.Schedule.Mode = "interval"
		if jobClone.Schedule.Mode != "cron" {
			t.Fatalf("cloned schedule mode = %q, want %q", jobClone.Schedule.Mode, "cron")
		}

		trigger := AutomationTriggerPreFirePayload{
			Payload: map[string]any{
				"outer":    map[string]any{"inner": "before"},
				"list":     []any{"before"},
				"typedMap": map[string][]string{"items": {"before"}},
				"typedList": []map[string]any{
					{"label": "before"},
				},
			},
		}
		triggerClone := cloneAsyncPayload(trigger)
		trigger.Payload["outer"].(map[string]any)["inner"] = "after"
		trigger.Payload["list"].([]any)[0] = "after"
		trigger.Payload["typedMap"].(map[string][]string)["items"][0] = "after"
		trigger.Payload["typedList"].([]map[string]any)[0]["label"] = "after"
		if triggerClone.Payload["outer"].(map[string]any)["inner"] != "before" {
			t.Fatalf("cloned nested payload = %#v, want preserved value", triggerClone.Payload["outer"])
		}
		if triggerClone.Payload["list"].([]any)[0] != "before" {
			t.Fatalf("cloned list payload = %#v, want preserved value", triggerClone.Payload["list"])
		}
		if triggerClone.Payload["typedMap"].(map[string][]string)["items"][0] != "before" {
			t.Fatalf("cloned typed map payload = %#v, want preserved value", triggerClone.Payload["typedMap"])
		}
		if triggerClone.Payload["typedList"].([]map[string]any)[0]["label"] != "before" {
			t.Fatalf("cloned typed list payload = %#v, want preserved value", triggerClone.Payload["typedList"])
		}
	})

	t.Run("Should agent payloads", func(t *testing.T) {
		t.Parallel()

		preStart := AgentPreStartPayload{Args: []string{"before"}}
		preStartClone := cloneAsyncPayload(preStart)
		preStart.Args[0] = "after"
		if preStartClone.Args[0] != "before" {
			t.Fatalf("cloned pre-start args = %#v, want preserved value", preStartClone.Args)
		}

		lifecycle := AgentLifecyclePayload{Args: []string{"before"}}
		lifecycleClone := cloneAsyncPayload(lifecycle)
		lifecycle.Args[0] = "after"
		if lifecycleClone.Args[0] != "before" {
			t.Fatalf("cloned lifecycle args = %#v, want preserved value", lifecycleClone.Args)
		}
	})

	t.Run("Should tool payloads", func(t *testing.T) {
		t.Parallel()

		preCall := ToolPreCallPayload{ToolInput: json.RawMessage(`{"tool":"before"}`)}
		preCallClone := cloneAsyncPayload(preCall)
		preCall.ToolInput[9] = 'X'
		if string(preCallClone.ToolInput) != `{"tool":"before"}` {
			t.Fatalf("cloned tool input = %s, want original payload", string(preCallClone.ToolInput))
		}

		postCall := ToolPostCallPayload{
			ToolInput:  json.RawMessage(`{"input":"before"}`),
			ToolResult: json.RawMessage(`{"result":"before"}`),
		}
		postCallClone := cloneAsyncPayload(postCall)
		postCall.ToolInput[10] = 'X'
		postCall.ToolResult[11] = 'X'
		if string(postCallClone.ToolInput) != `{"input":"before"}` {
			t.Fatalf("cloned post-call input = %s, want original payload", string(postCallClone.ToolInput))
		}
		if string(postCallClone.ToolResult) != `{"result":"before"}` {
			t.Fatalf("cloned post-call result = %s, want original payload", string(postCallClone.ToolResult))
		}

		postError := ToolPostErrorPayload{ToolInput: json.RawMessage(`{"error":"before"}`)}
		postErrorClone := cloneAsyncPayload(postError)
		postError.ToolInput[10] = 'X'
		if string(postErrorClone.ToolInput) != `{"error":"before"}` {
			t.Fatalf("cloned post-error input = %s, want original payload", string(postErrorClone.ToolInput))
		}
	})

	t.Run("Should permission payloads", func(t *testing.T) {
		t.Parallel()

		request := PermissionRequestPayload{
			ToolInput: json.RawMessage(`{"decision":"before"}`),
			ToolCall: PermissionToolCall{
				Locations: []ToolLocation{{Path: "/tmp/before"}},
			},
			Options: []PermissionOption{{Decision: "before"}},
		}
		requestClone := cloneAsyncPayload(request)
		request.ToolInput[13] = 'X'
		request.ToolCall.Locations[0].Path = "/tmp/after"
		request.Options[0].Decision = "after"
		if string(requestClone.ToolInput) != `{"decision":"before"}` {
			t.Fatalf("cloned permission input = %s, want original payload", string(requestClone.ToolInput))
		}
		if requestClone.ToolCall.Locations[0].Path != "/tmp/before" {
			t.Fatalf("cloned permission location = %q, want %q", requestClone.ToolCall.Locations[0].Path, "/tmp/before")
		}
		if requestClone.Options[0].Decision != "before" {
			t.Fatalf("cloned permission option = %q, want %q", requestClone.Options[0].Decision, "before")
		}

		resolution := PermissionResolutionPayload{
			ToolInput: json.RawMessage(`{"resolution":"before"}`),
			ToolCall: PermissionToolCall{
				Locations: []ToolLocation{{Path: "/tmp/resolution-before"}},
			},
		}
		resolutionClone := cloneAsyncPayload(resolution)
		resolution.ToolInput[15] = 'X'
		resolution.ToolCall.Locations[0].Path = "/tmp/resolution-after"
		if string(resolutionClone.ToolInput) != `{"resolution":"before"}` {
			t.Fatalf("cloned resolution input = %s, want original payload", string(resolutionClone.ToolInput))
		}
		if resolutionClone.ToolCall.Locations[0].Path != "/tmp/resolution-before" {
			t.Fatalf(
				"cloned resolution location = %q, want %q",
				resolutionClone.ToolCall.Locations[0].Path,
				"/tmp/resolution-before",
			)
		}
	})

	t.Run("Should default branch returns value", func(t *testing.T) {
		t.Parallel()

		original := SessionPreCreatePayload{SessionContext: SessionContext{SessionID: "session-default"}}
		cloned := cloneAsyncPayload(original)
		if cloned.SessionID != original.SessionID {
			t.Fatalf("cloned session id = %q, want %q", cloned.SessionID, original.SessionID)
		}
	})

	t.Run("Should clone session soul context for async payloads", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			clone func(*SessionSoulContext) *SessionSoulContext
		}{
			{
				name: "Should clone input pre-submit session soul context",
				clone: func(soul *SessionSoulContext) *SessionSoulContext {
					cloned := cloneAsyncPayload(InputPreSubmitPayload{
						SessionContext: SessionContext{SessionSoulContext: soul},
					})
					return cloned.SessionSoulContext
				},
			},
			{
				name: "Should clone session pre-create soul context",
				clone: func(soul *SessionSoulContext) *SessionSoulContext {
					cloned := cloneAsyncPayload(SessionPreCreatePayload{
						SessionContext: SessionContext{SessionSoulContext: soul},
					})
					return cloned.SessionSoulContext
				},
			},
			{
				name: "Should clone session lifecycle soul context",
				clone: func(soul *SessionSoulContext) *SessionSoulContext {
					cloned := cloneAsyncPayload(SessionLifecyclePayload{
						SessionContext: SessionContext{SessionSoulContext: soul},
					})
					return cloned.SessionSoulContext
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				original := &SessionSoulContext{
					SoulSnapshotID: "snap-before",
					SoulDigest:     "digest-before",
				}
				cloned := tt.clone(original)

				original.SoulSnapshotID = "snap-after"
				original.SoulDigest = "digest-after"

				if cloned == nil {
					t.Fatal("cloned SessionSoulContext = nil, want snapshot")
				}
				if cloned == original {
					t.Fatal("cloned SessionSoulContext shares original pointer")
				}
				if cloned.SoulSnapshotID != "snap-before" {
					t.Fatalf("cloned soul snapshot id = %q, want snap-before", cloned.SoulSnapshotID)
				}
				if cloned.SoulDigest != "digest-before" {
					t.Fatalf("cloned soul digest = %q, want digest-before", cloned.SoulDigest)
				}
			})
		}
	})
}

func TestCloneAsyncPayloadCopiesTaskAndSpawnReferences(t *testing.T) {
	t.Parallel()

	t.Run("Should task pre claim criteria", func(t *testing.T) {
		t.Parallel()

		original := TaskRunPreClaimPayload{
			TaskRunContext: &TaskRunContext{
				ResolvedNetworkParticipation: participation.CloneSpec(participation.Spec{
					ChannelID: "operations",
				}),
			},
			Criteria: TaskRunClaimCriteria{
				RequiredCapabilities: []string{"network"},
			},
		}
		cloned := cloneAsyncPayload(original)
		original.Criteria.RequiredCapabilities[0] = "mutated"
		original.ResolvedNetworkParticipation.ChannelID = "mutated"

		if cloned.Criteria.RequiredCapabilities[0] != "network" {
			t.Fatalf(
				"cloned required capability = %q, want %q",
				cloned.Criteria.RequiredCapabilities[0],
				"network",
			)
		}
		if cloned.ResolvedNetworkParticipation.ChannelID != "operations" {
			t.Fatalf(
				"cloned network channel = %q, want %q",
				cloned.ResolvedNetworkParticipation.ChannelID,
				"operations",
			)
		}
	})

	t.Run("Should task value payloads", func(t *testing.T) {
		t.Parallel()

		enqueued := cloneAsyncPayload(TaskRunEnqueuedPayload{TaskRunContext: TaskRunContext{TaskID: "task-enqueued"}})
		postClaim := cloneAsyncPayload(TaskRunPostClaimPayload{TaskRunContext: TaskRunContext{TaskID: "task-claim"}})
		lease := cloneAsyncPayload(TaskRunLeasePayload{TaskRunContext: TaskRunContext{TaskID: "task-lease"}})

		if enqueued.TaskID != "task-enqueued" {
			t.Fatalf("enqueued task id = %q, want task-enqueued", enqueued.TaskID)
		}
		if postClaim.TaskID != "task-claim" {
			t.Fatalf("post-claim task id = %q, want task-claim", postClaim.TaskID)
		}
		if lease.TaskID != "task-lease" {
			t.Fatalf("lease task id = %q, want task-lease", lease.TaskID)
		}
	})

	t.Run("Should spawn permissions", func(t *testing.T) {
		t.Parallel()

		original := SpawnPreCreatePayload{
			ParentPermissions: &PermissionSet{
				Tools:           []string{"tool.before"},
				Skills:          []string{"skill.before"},
				MCPServers:      []string{"mcp.before"},
				WorkspacePaths:  []string{"/workspace/before"},
				NetworkChannels: []string{"network.before"},
				SandboxProfiles: []string{"sandbox.before"},
			},
			ChildPermissions: &PermissionSet{Tools: []string{"child.before"}},
		}
		cloned := cloneAsyncPayload(original)
		original.ParentPermissions.Tools[0] = "tool.after"
		original.ParentPermissions.Skills[0] = "skill.after"
		original.ParentPermissions.MCPServers[0] = "mcp.after"
		original.ParentPermissions.WorkspacePaths[0] = "/workspace/after"
		original.ParentPermissions.NetworkChannels[0] = "network.after"
		original.ParentPermissions.SandboxProfiles[0] = "sandbox.after"
		original.ChildPermissions.Tools[0] = "child.after"

		if cloned.ParentPermissions.Tools[0] != "tool.before" {
			t.Fatalf("cloned parent tools = %#v, want preserved tools", cloned.ParentPermissions.Tools)
		}
		if cloned.ParentPermissions.Skills[0] != "skill.before" {
			t.Fatalf("cloned parent skills = %#v, want preserved skills", cloned.ParentPermissions.Skills)
		}
		if cloned.ParentPermissions.MCPServers[0] != "mcp.before" {
			t.Fatalf("cloned parent MCP servers = %#v, want preserved servers", cloned.ParentPermissions.MCPServers)
		}
		if cloned.ParentPermissions.WorkspacePaths[0] != "/workspace/before" {
			t.Fatalf("cloned workspace paths = %#v, want preserved paths", cloned.ParentPermissions.WorkspacePaths)
		}
		if cloned.ParentPermissions.NetworkChannels[0] != "network.before" {
			t.Fatalf("cloned network channels = %#v, want preserved channels", cloned.ParentPermissions.NetworkChannels)
		}
		if cloned.ParentPermissions.SandboxProfiles[0] != "sandbox.before" {
			t.Fatalf("cloned sandbox profiles = %#v, want preserved profiles", cloned.ParentPermissions.SandboxProfiles)
		}
		if cloned.ChildPermissions.Tools[0] != "child.before" {
			t.Fatalf("cloned child tools = %#v, want preserved tools", cloned.ChildPermissions.Tools)
		}
	})

	t.Run("Should spawn lifecycle nil permissions", func(t *testing.T) {
		t.Parallel()

		cloned := cloneAsyncPayload(SpawnLifecyclePayload{})
		if cloned.ParentPermissions != nil {
			t.Fatalf("parent permissions = %#v, want nil", cloned.ParentPermissions)
		}
		if cloned.ChildPermissions != nil {
			t.Fatalf("child permissions = %#v, want nil", cloned.ChildPermissions)
		}
	})

	t.Run("Should isolate every window-manager entity change slice", func(t *testing.T) {
		t.Parallel()

		original := windowManagerDispatchTestPayload(HookWindowManagerWindowMoved)
		cloned := cloneAsyncPayload(original)
		cloned.Changes.DesktopIDs[0] = "desktop-mutated"
		cloned.Changes.WindowIDs[0] = "window-mutated"
		cloned.Changes.GroupIDs[0] = "group-mutated"
		cloned.Changes.NodeIDs[0] = "node-mutated"
		cloned.Changes.ClientIDs[0] = "client-mutated"

		if original.Changes.DesktopIDs[0] != "desktop-a" ||
			original.Changes.WindowIDs[0] != "window-a" ||
			original.Changes.GroupIDs[0] != "group-a" ||
			original.Changes.NodeIDs[0] != "node-a" ||
			original.Changes.ClientIDs[0] != "client-a" {
			t.Fatalf("cloneAsyncPayload() aliased original changes: %#v", original.Changes)
		}
	})
}
