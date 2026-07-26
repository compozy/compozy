package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
)

func TestMeCommandJSONReturnsValidatedIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should return validated identity as JSON", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{}
		deps := newAgentCommandTestDeps(t, client)
		client.agentMeFn = func(_ context.Context, credentials agentidentity.Credentials) (AgentMeRecord, error) {
			assertAgentCredentials(t, credentials)
			return AgentMeRecord{
				Self: contract.AgentIdentityPayload{
					SessionID: "sess-agent",
					AgentName: "coder",
					Provider:  "test-provider",
					Model:     "test-model",
				},
				Workspace: contract.AgentWorkspacePayload{
					ID:      "ws-1",
					RootDir: "/workspace/project",
				},
				Session: contract.AgentSessionPayload{
					ID:                           "sess-agent",
					State:                        session.StateActive,
					ResolvedNetworkParticipation: testLiveResolvedParticipation("builders"),
					CreatedAt:                    fixedTestNow,
					UpdatedAt:                    fixedTestNow,
				},
			}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "me", "-o", "json")
		if err != nil {
			t.Fatalf("agh me error = %v", err)
		}

		var got AgentMeRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(agh me) error = %v", err)
		}
		if got.Self.SessionID != "sess-agent" || got.Self.AgentName != "coder" || got.Workspace.ID != "ws-1" {
			t.Fatalf("agh me payload = %#v, want caller session/workspace identity", got)
		}
	})
}

func TestMeContextCommandJSONKeepsStableSectionOrder(t *testing.T) {
	t.Parallel()

	t.Run("Should keep stable JSON section order", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{}
		deps := newAgentCommandTestDeps(t, client)
		client.agentContextFn = func(_ context.Context, credentials agentidentity.Credentials) (AgentContextRecord, error) {
			assertAgentCredentials(t, credentials)
			return AgentContextRecord{
				Self: contract.AgentIdentityPayload{
					SessionID: "sess-agent",
					AgentName: "coder",
					Provider:  "test-provider",
				},
				Workspace: contract.AgentWorkspacePayload{ID: "ws-1", RootDir: "/workspace/project"},
				Session: contract.AgentSessionPayload{
					ID:        "sess-agent",
					State:     session.StateActive,
					CreatedAt: fixedTestNow,
					UpdatedAt: fixedTestNow,
				},
				Task:                contract.AgentTaskContextPayload{Available: true},
				CoordinationChannel: contract.AgentCoordinationChannelContextPayload{Available: true},
				InboxSummary:        contract.AgentInboxSummaryPayload{},
				PeerRoster:          contract.AgentPeerRosterPayload{},
				Capabilities:        contract.AgentCapabilitySectionPayload{},
				Limits:              contract.AgentLimitsPayload{ContextSectionLimit: 20},
				Provenance: contract.AgentContextProvenancePayload{
					GeneratedAt: fixedTestNow,
					Source:      "test",
				},
			}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "me", "context", "-o", "json")
		if err != nil {
			t.Fatalf("agh me context error = %v", err)
		}

		assertJSONKeyOrder(t, stdout, []string{
			"self",
			"workspace",
			"session",
			"task",
			"coordination_channel",
			"inbox_summary",
			"peer_roster",
			"capabilities",
			"limits",
			"provenance",
		})
	})
}

func TestSpawnCommandMapsBoundedChildRequest(t *testing.T) {
	t.Parallel()

	t.Run("Should map bounded child request", func(t *testing.T) {
		t.Parallel()

		var gotRequest AgentSpawnRequest
		client := &stubClient{}
		deps := newAgentCommandTestDeps(t, client)
		client.agentSpawnFn = func(
			_ context.Context,
			request AgentSpawnRequest,
			credentials agentidentity.Credentials,
		) (AgentSpawnRecord, error) {
			assertAgentCredentials(t, credentials)
			gotRequest = request
			ttl := fixedTestNow.Add(2 * time.Minute)
			return AgentSpawnRecord{
				Session: SessionRecord{
					ID:            "sess-child",
					Name:          request.Name,
					AgentName:     request.AgentName,
					Provider:      request.Provider,
					WorkspaceID:   "ws-1",
					WorkspacePath: "/workspace/project",
					Type:          session.SessionTypeSpawned,
					State:         session.StateActive,
					CreatedAt:     fixedTestNow,
					UpdatedAt:     fixedTestNow,
				},
				Lineage: contract.SessionLineagePayload{
					ParentSessionID:  "sess-agent",
					RootSessionID:    "sess-agent",
					SpawnDepth:       1,
					SpawnRole:        request.SpawnRole,
					TTLExpiresAt:     &ttl,
					AutoStopOnParent: request.AutoStopOnParent,
					SpawnBudget: contract.SpawnBudgetPayload{
						MaxChildren: 5,
						MaxDepth:    1,
						TTLSeconds:  request.TTLSeconds,
					},
					PermissionPolicy: request.Permissions,
				},
				Permissions: request.Permissions,
			}, nil
		}

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"spawn",
			"--agent",
			"coder",
			"--provider",
			"codex",
			"--model",
			"gpt-test",
			"--name",
			"child",
			"--prompt-overlay",
			"focus",
			"--role",
			"worker",
			"--ttl-seconds",
			"120",
			"--tool",
			"read",
			"--skill",
			"go",
			"--mcp-server",
			"filesystem",
			"--workspace-path",
			"/workspace/project",
			"--channel",
			"builders",
			"--sandbox-profile",
			"default",
			"--idempotency-key",
			"spawn-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("agh spawn error = %v", err)
		}
		if gotRequest.AgentName != "coder" ||
			gotRequest.Provider != "codex" ||
			gotRequest.Model != "gpt-test" ||
			gotRequest.Name != "child" ||
			gotRequest.PromptOverlay != "focus" ||
			gotRequest.SpawnRole != "worker" ||
			gotRequest.TTLSeconds != 120 ||
			!gotRequest.AutoStopOnParent ||
			gotRequest.IdempotencyKey != "spawn-1" {
			t.Fatalf("spawn request = %#v, want parsed bounded spawn request", gotRequest)
		}
		if len(gotRequest.Permissions.Tools) != 1 ||
			gotRequest.Permissions.Tools[0] != "read" ||
			len(gotRequest.Permissions.Skills) != 1 ||
			gotRequest.Permissions.Skills[0] != "go" ||
			len(gotRequest.Permissions.MCPServers) != 1 ||
			gotRequest.Permissions.MCPServers[0] != "filesystem" ||
			len(gotRequest.Permissions.WorkspacePaths) != 1 ||
			gotRequest.Permissions.WorkspacePaths[0] != "/workspace/project" ||
			len(gotRequest.Permissions.NetworkChannels) != 1 ||
			gotRequest.Permissions.NetworkChannels[0] != "builders" ||
			len(gotRequest.Permissions.SandboxProfiles) != 1 ||
			gotRequest.Permissions.SandboxProfiles[0] != "default" {
			t.Fatalf("spawn permissions = %#v, want all repeatable atom flags", gotRequest.Permissions)
		}

		var output AgentSpawnRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(spawn output) error = %v", err)
		}
		if output.Session.ID != "sess-child" || output.Lineage.ParentSessionID != "sess-agent" {
			t.Fatalf("spawn output = %#v, want child session with parent lineage", output)
		}
	})
}

func TestChannelSendRejectsMissingInputsAndInvalidIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		deps func(t *testing.T, client *stubClient) commandDeps
		args []string
	}{
		{
			name: "Should reject missing channel",
			deps: newAgentCommandTestDeps,
			args: []string{"ch", "send", "--body", `{"text":"ok"}`, "--task-id", "task-1", "--run-id", "run-1"},
		},
		{
			name: "Should reject missing body",
			deps: newAgentCommandTestDeps,
			args: []string{"ch", "send", "builders", "--task-id", "task-1", "--run-id", "run-1"},
		},
		{
			name: "Should reject invalid caller identity",
			deps: newMissingAgentIdentityDeps,
			args: []string{
				"ch", "send", "builders",
				"--body", `{"text":"ok"}`,
				"--task-id", "task-1",
				"--run-id", "run-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &stubClient{
				agentChannelSendFn: func(
					context.Context,
					string,
					AgentChannelSendRequest,
					agentidentity.Credentials,
				) (AgentChannelMessageRecord, error) {
					t.Fatal("AgentChannelSend should not be called for invalid input")
					return AgentChannelMessageRecord{}, errors.New("unexpected")
				},
			}
			_, _, err := executeRootCommand(t, tt.deps(t, client), tt.args...)
			if err == nil {
				t.Fatal("agh ch send error = nil, want validation/identity error")
			}
		})
	}
}

func TestAgentCommandsRejectMissingIdentityBeforeAgentCalls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "Should reject me without identity", args: []string{"me", "-o", "json"}},
		{name: "Should reject me context without identity", args: []string{"me", "context", "-o", "json"}},
		{name: "Should reject ch list without identity", args: []string{"ch", "list", "-o", "json"}},
		{name: "Should reject ch recv without identity", args: []string{"ch", "recv", "builders", "-o", "json"}},
		{name: "Should reject task next without identity", args: []string{"task", "next", "-o", "json"}},
		{
			name: "Should reject task heartbeat without identity",
			args: []string{"task", "heartbeat", "run-1", "-o", "json"},
		},
		{
			name: "Should reject task complete without identity",
			args: []string{"task", "complete", "run-1", "-o", "json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &stubClient{}
			_, _, err := executeRootCommand(t, newMissingAgentIdentityDeps(t, client), tt.args...)
			if !errors.Is(err, agentidentity.ErrIdentityRequired) {
				t.Fatalf("executeRootCommand(%v) error = %v, want ErrIdentityRequired", tt.args, err)
			}
		})
	}
}

func TestChannelListCommandJSONReturnsVisibleChannels(t *testing.T) {
	t.Parallel()

	t.Run("Should return visible channels as JSON", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{}
		deps := newAgentCommandTestDeps(t, client)
		client.agentChannelsFn = func(_ context.Context, credentials agentidentity.Credentials) ([]AgentChannelRecord, error) {
			assertAgentCredentials(t, credentials)
			return []AgentChannelRecord{{
				ID:                  "builders",
				DisplayName:         "builders",
				Purpose:             "task_coordination",
				WorkspaceID:         "ws-1",
				AllowedMessageKinds: contract.CoordinationMessageKinds(),
			}}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "ch", "list", "-o", "json")
		if err != nil {
			t.Fatalf("agh ch list error = %v", err)
		}

		var channels []AgentChannelRecord
		if err := json.Unmarshal([]byte(stdout), &channels); err != nil {
			t.Fatalf("json.Unmarshal(channels) error = %v", err)
		}
		if len(channels) != 1 ||
			channels[0].ID != "builders" ||
			len(channels[0].AllowedMessageKinds) != len(contract.CoordinationMessageKinds()) {
			t.Fatalf("channels = %#v, want builders with MVP message kinds", channels)
		}
	})
}

func TestChannelSendPreservesCoordinationMetadataAndRejectsClaimToken(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve coordination metadata and reject raw claim tokens", func(t *testing.T) {
		t.Parallel()

		for _, kind := range []contract.CoordinationMessageKind{
			contract.CoordinationMessageStatus,
			contract.CoordinationMessageBlocker,
			contract.CoordinationMessageResult,
		} {
			t.Run("Should preserve "+string(kind)+" coordination metadata", func(t *testing.T) {
				t.Parallel()

				client := &stubClient{}
				deps := newAgentCommandTestDeps(t, client)
				client.agentChannelSendFn = func(
					_ context.Context,
					channel string,
					request AgentChannelSendRequest,
					credentials agentidentity.Credentials,
				) (AgentChannelMessageRecord, error) {
					assertAgentCredentials(t, credentials)
					if channel != "builders" {
						t.Fatalf("channel = %q, want builders", channel)
					}
					if request.Metadata.TaskID != "task-1" ||
						request.Metadata.RunID != "run-1" ||
						request.Metadata.WorkflowID != "wf-1" ||
						request.Metadata.ChannelID != "builders" ||
						request.Metadata.CorrelationID != "corr-1" ||
						request.Metadata.MessageKind != kind {
						t.Fatalf("metadata = %#v, want task/run/%s correlation", request.Metadata, kind)
					}
					if string(request.Metadata.Ext["note"]) != `"safe"` {
						t.Fatalf("metadata.Ext = %#v, want note", request.Metadata.Ext)
					}
					if request.IdempotencyKey != "idem-1" {
						t.Fatalf("idempotency key = %q, want idem-1", request.IdempotencyKey)
					}
					return AgentChannelMessageRecord{
						MessageID: "msg-1",
						ChannelID: "builders",
						Body:      request.Body,
						Metadata:  request.Metadata,
						Timestamp: fixedTestNow,
					}, nil
				}

				_, _, err := executeRootCommand(
					t,
					deps,
					"ch", "send", "builders",
					"--body", `{"text":"ok"}`,
					"--task-id", "task-1",
					"--run-id", "run-1",
					"--workflow-id", "wf-1",
					"--kind", string(kind),
					"--correlation-id", "corr-1",
					"--metadata-ext", `{"note":"safe"}`,
					"--idempotency-key", "idem-1",
					"-o", "json",
				)
				if err != nil {
					t.Fatalf("agh ch send error = %v", err)
				}
			})
		}

		for _, tt := range []struct {
			name string
			args []string
		}{
			{
				name: "Should reject raw claim token in body",
				args: []string{
					"ch", "send", "builders",
					"--body", `{"claim_token":"agh_claim_CLI_CHANNEL_123"}`,
					"--task-id", "task-1",
					"--run-id", "run-1",
				},
			},
			{
				name: "Should reject raw claim token in metadata ext",
				args: []string{
					"ch", "send", "builders",
					"--body", `{"text":"ok"}`,
					"--task-id", "task-1",
					"--run-id", "run-1",
					"--metadata-ext", `{"claim_token":"agh_claim_CLI_CHANNEL_123"}`,
				},
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				client := &stubClient{
					agentChannelSendFn: func(
						context.Context,
						string,
						AgentChannelSendRequest,
						agentidentity.Credentials,
					) (AgentChannelMessageRecord, error) {
						t.Fatal("AgentChannelSend should not be called when claim_token is present")
						return AgentChannelMessageRecord{}, errors.New("unexpected")
					},
				}
				_, _, err := executeRootCommand(t, newAgentCommandTestDeps(t, client), tt.args...)
				if !errors.Is(err, contract.ErrRawClaimTokenMetadata) {
					t.Fatalf("agh ch send error = %v, want ErrRawClaimTokenMetadata", err)
				}
				if strings.Contains(err.Error(), "agh_claim_CLI_CHANNEL_123") {
					t.Fatalf("agh ch send error leaked raw claim token: %v", err)
				}
			})
		}

		identityClient := &stubClient{
			agentChannelSendFn: func(
				context.Context,
				string,
				AgentChannelSendRequest,
				agentidentity.Credentials,
			) (AgentChannelMessageRecord, error) {
				t.Fatal("AgentChannelSend should not be called for an unsupported caller identity flag")
				return AgentChannelMessageRecord{}, errors.New("unexpected")
			},
		}
		_, _, err := executeRootCommand(
			t,
			newAgentCommandTestDeps(t, identityClient),
			"ch", "send", "builders",
			"--body", `{"text":"spoof"}`,
			"--task-id", "task-1",
			"--run-id", "run-1",
			"--from", "alice@39f713d0a644253f04529421b9f51b9b",
		)
		if err == nil || !strings.Contains(err.Error(), "unknown flag: --from") {
			t.Fatalf("agh ch send caller identity error = %v, want unsupported identity field rejection", err)
		}
	})
}

func TestChannelReplySendsOnlyMessageIDAndBodyWhenMetadataIsResolvedServerSide(t *testing.T) {
	t.Parallel()

	t.Run("Should send only message ID and body when metadata is server-resolved", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{}
		deps := newAgentCommandTestDeps(t, client)
		replyCalls := 0
		client.agentChannelReplyFn = func(
			_ context.Context,
			request AgentChannelReplyRequest,
			credentials agentidentity.Credentials,
		) (AgentChannelMessageRecord, error) {
			replyCalls++
			assertAgentCredentials(t, credentials)
			if request.ReplyToMessageID != "msg-source" {
				t.Fatalf("reply_to_message_id = %q, want msg-source", request.ReplyToMessageID)
			}
			if string(request.Body) != `{"text":"ack"}` {
				t.Fatalf("body = %s, want ack JSON", request.Body)
			}
			switch replyCalls {
			case 1:
				if !zeroCLICoordinationMetadata(request.Metadata) {
					t.Fatalf("metadata = %#v, want zero metadata for server-side source resolution", request.Metadata)
				}
			case 2:
				if request.Metadata.TaskID != "task-1" ||
					request.Metadata.RunID != "run-1" ||
					request.Metadata.ChannelID != "builders" ||
					request.Metadata.CorrelationID != "corr-1" ||
					request.Metadata.MessageKind != contract.CoordinationMessageReply {
					t.Fatalf("metadata = %#v, want task/run/channel/correlation reply metadata", request.Metadata)
				}
			default:
				t.Fatalf("reply callback calls = %d, want at most 2", replyCalls)
			}
			return AgentChannelMessageRecord{
				MessageID: "msg-reply",
				ChannelID: "builders",
				Body:      request.Body,
				Metadata: contract.CoordinationMessageMetadataPayload{
					TaskID:        "task-1",
					RunID:         "run-1",
					ChannelID:     "builders",
					MessageKind:   contract.CoordinationMessageReply,
					CorrelationID: "run-1",
				},
				Timestamp: fixedTestNow,
			}, nil
		}

		if _, _, err := executeRootCommand(
			t,
			deps,
			"ch", "reply",
			"--to-message", "msg-source",
			"--body", `{"text":"ack"}`,
			"-o", "json",
		); err != nil {
			t.Fatalf("agh ch reply error = %v", err)
		}
		if _, _, err := executeRootCommand(
			t,
			deps,
			"ch", "reply",
			"--to-message", "msg-source",
			"--body", `{"text":"ack"}`,
			"--task-id", "task-1",
			"--run-id", "run-1",
			"--channel-id", "builders",
			"--correlation-id", "corr-1",
			"-o", "json",
		); err != nil {
			t.Fatalf("agh ch reply with metadata error = %v", err)
		}

		_, _, err := executeRootCommand(
			t,
			deps,
			"ch", "reply",
			"--to-message", "msg-source",
			"--body", `{"text":"ack"}`,
			"--kind", "status",
		)
		if err == nil || !strings.Contains(err.Error(), "--kind must be reply") {
			t.Fatalf("agh ch reply --kind status error = %v, want reply-kind validation", err)
		}

		_, _, err = executeRootCommand(
			t,
			deps,
			"ch", "reply",
			"--to-message", "msg-source",
			"--body", `{"text":"ack"}`,
			"--task-id", "task-1",
			"--run-id", "run-1",
			"--channel-id", "builders",
			"--kind", "status",
		)
		if err == nil || !strings.Contains(err.Error(), "--kind must be reply") {
			t.Fatalf("agh ch reply --kind status error = %v, want reply-kind validation", err)
		}
	})
}

func TestChannelRecvJSONLOutputEmitsOneObjectPerMessage(t *testing.T) {
	t.Parallel()

	t.Run("Should emit one JSONL object per message", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{}
		deps := newAgentCommandTestDeps(t, client)
		client.agentChannelRecvFn = func(
			_ context.Context,
			channel string,
			query AgentChannelRecvQuery,
			credentials agentidentity.Credentials,
		) ([]AgentChannelMessageRecord, error) {
			assertAgentCredentials(t, credentials)
			if channel != "builders" || !query.Wait || query.Limit != 2 {
				t.Fatalf("recv channel/query = %q/%#v, want builders wait limit=2", channel, query)
			}
			return []AgentChannelMessageRecord{
				agentChannelTestMessage("msg-1", contract.CoordinationMessageStatus),
				agentChannelTestMessage("msg-2", contract.CoordinationMessageResult),
			}, nil
		}

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"ch", "recv", "builders",
			"--wait",
			"--limit", "2",
			"-o", "jsonl",
		)
		if err != nil {
			t.Fatalf("agh ch recv error = %v", err)
		}

		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 2 {
			t.Fatalf("jsonl line count = %d, want 2; output=%q", len(lines), stdout)
		}
		for index, line := range lines {
			var message AgentChannelMessageRecord
			if err := json.Unmarshal([]byte(line), &message); err != nil {
				t.Fatalf("json.Unmarshal(line %d) error = %v", index, err)
			}
			if message.MessageID == "" || message.Metadata.MessageKind == "" {
				t.Fatalf("message line %d = %#v, want populated message", index, message)
			}
		}
	})
}

func TestAgentCommandsRenderHumanAndToonOutputs(t *testing.T) {
	t.Parallel()

	t.Run("Should render human and toon outputs", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{}
		deps := newAgentCommandTestDeps(t, client)
		meRecord := AgentMeRecord{
			Self: contract.AgentIdentityPayload{
				SessionID: "sess-agent",
				AgentName: "coder",
				Provider:  "test-provider",
				Model:     "test-model",
			},
			Workspace: contract.AgentWorkspacePayload{ID: "ws-1", RootDir: "/workspace/project"},
			Session: contract.AgentSessionPayload{
				ID:        "sess-agent",
				State:     session.StateActive,
				CreatedAt: fixedTestNow,
				UpdatedAt: fixedTestNow,
			},
		}
		contextRecord := AgentContextRecord{
			Self:      meRecord.Self,
			Workspace: meRecord.Workspace,
			Session:   meRecord.Session,
			Provenance: contract.AgentContextProvenancePayload{
				GeneratedAt: fixedTestNow,
				Source:      "test",
			},
		}
		channelRecord := AgentChannelRecord{
			ID:                  "builders",
			DisplayName:         "builders",
			Purpose:             "task_coordination",
			WorkspaceID:         "ws-1",
			AllowedMessageKinds: contract.CoordinationMessageKinds(),
		}
		statusMessage := agentChannelTestMessage("msg-1", contract.CoordinationMessageStatus)
		replyMessage := agentChannelTestMessage("msg-reply", contract.CoordinationMessageReply)

		client.agentMeFn = func(context.Context, agentidentity.Credentials) (AgentMeRecord, error) {
			return meRecord, nil
		}
		client.agentContextFn = func(context.Context, agentidentity.Credentials) (AgentContextRecord, error) {
			return contextRecord, nil
		}
		client.agentChannelsFn = func(context.Context, agentidentity.Credentials) ([]AgentChannelRecord, error) {
			return []AgentChannelRecord{channelRecord}, nil
		}
		client.agentChannelRecvFn = func(
			context.Context,
			string,
			AgentChannelRecvQuery,
			agentidentity.Credentials,
		) ([]AgentChannelMessageRecord, error) {
			return []AgentChannelMessageRecord{statusMessage}, nil
		}
		client.agentChannelSendFn = func(
			context.Context,
			string,
			AgentChannelSendRequest,
			agentidentity.Credentials,
		) (AgentChannelMessageRecord, error) {
			return statusMessage, nil
		}
		client.agentChannelReplyFn = func(
			context.Context,
			AgentChannelReplyRequest,
			agentidentity.Credentials,
		) (AgentChannelMessageRecord, error) {
			return replyMessage, nil
		}

		tests := []struct {
			name string
			args []string
			want string
		}{
			{name: "Should render me human output", args: []string{"me", "-o", "human"}, want: "Agent"},
			{name: "Should render me toon output", args: []string{"me", "-o", "toon"}, want: "agent_me{session_id"},
			{
				name: "Should render context human output",
				args: []string{"me", "context", "-o", "human"},
				want: `"source": "test"`,
			},
			{
				name: "Should render context toon output",
				args: []string{"me", "context", "-o", "toon"},
				want: `"source": "test"`,
			},
			{
				name: "Should render channels human output",
				args: []string{"ch", "list", "-o", "human"},
				want: "Agent Channels",
			},
			{
				name: "Should render channels toon output",
				args: []string{"ch", "list", "-o", "toon"},
				want: "agent_channels[1]",
			},
			{
				name: "Should render recv human output",
				args: []string{"ch", "recv", "builders", "-o", "human"},
				want: "Agent Channel Messages",
			},
			{
				name: "Should render recv toon output",
				args: []string{"ch", "recv", "builders", "-o", "toon"},
				want: "agent_channel_messages[1]",
			},
			{
				name: "Should render send human output",
				args: []string{
					"ch", "send", "builders",
					"--body", `{"text":"ok"}`,
					"--task-id", "task-1",
					"--run-id", "run-1",
					"-o", "human",
				},
				want: "Agent Channel Message",
			},
			{
				name: "Should render reply toon output",
				args: []string{
					"ch", "reply",
					"--to-message", "msg-1",
					"--body", `{"text":"ack"}`,
					"-o", "toon",
				},
				want: "agent_channel_message{message_id",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				stdout, _, err := executeRootCommand(t, deps, tt.args...)
				if err != nil {
					t.Fatalf("executeRootCommand(%v) error = %v", tt.args, err)
				}
				if !strings.Contains(stdout, tt.want) {
					t.Fatalf("output = %q, want substring %q", stdout, tt.want)
				}
			})
		}
	})
}

func newAgentCommandTestDeps(t *testing.T, client *stubClient) commandDeps {
	t.Helper()

	client.getSessionFn = func(_ context.Context, id string) (SessionRecord, error) {
		if id != "sess-agent" {
			return SessionRecord{}, session.ErrSessionNotFound
		}
		return agentCommandSessionRecord(), nil
	}
	deps := newTestDeps(t, client)
	deps.getenv = agentCommandEnv
	return deps
}

func newMissingAgentIdentityDeps(t *testing.T, client *stubClient) commandDeps {
	t.Helper()

	client.getSessionFn = func(context.Context, string) (SessionRecord, error) {
		t.Fatal("GetSession should not be called when agent env identity is missing")
		return SessionRecord{}, errors.New("unexpected")
	}
	return newTestDeps(t, client)
}

func agentCommandEnv(key string) string {
	switch key {
	case agentidentity.EnvSessionID:
		return "sess-agent"
	case agentidentity.EnvAgent:
		return "coder"
	default:
		return ""
	}
}

func agentCommandSessionRecord() SessionRecord {
	return SessionRecord{
		ID:            "sess-agent",
		Name:          "worker",
		AgentName:     "coder",
		Provider:      "test-provider",
		WorkspaceID:   "ws-1",
		WorkspacePath: "/workspace/project",
		Type:          session.SessionTypeUser,
		State:         session.StateActive,
		CreatedAt:     fixedTestNow,
		UpdatedAt:     fixedTestNow,
	}
}

func assertAgentCredentials(t *testing.T, credentials agentidentity.Credentials) {
	t.Helper()

	if credentials.SessionID != "sess-agent" ||
		credentials.AgentName != "coder" ||
		credentials.WorkspaceID != "" {
		t.Fatalf("credentials = %#v, want validated agent env identity", credentials)
	}
}

func assertJSONKeyOrder(t *testing.T, output string, keys []string) {
	t.Helper()

	previousIndex := -1
	for _, key := range keys {
		index := strings.Index(output, `"`+key+`"`)
		if index < 0 {
			t.Fatalf("JSON output missing key %q: %s", key, output)
		}
		if index <= previousIndex {
			t.Fatalf("JSON key %q appears out of order in %s", key, output)
		}
		previousIndex = index
	}
}

func zeroCLICoordinationMetadata(metadata contract.CoordinationMessageMetadataPayload) bool {
	return strings.TrimSpace(metadata.TaskID) == "" &&
		strings.TrimSpace(metadata.RunID) == "" &&
		strings.TrimSpace(metadata.WorkflowID) == "" &&
		strings.TrimSpace(metadata.ChannelID) == "" &&
		strings.TrimSpace(string(metadata.MessageKind)) == "" &&
		strings.TrimSpace(metadata.CorrelationID) == "" &&
		len(metadata.Ext) == 0
}

func agentChannelTestMessage(id string, kind contract.CoordinationMessageKind) AgentChannelMessageRecord {
	return AgentChannelMessageRecord{
		MessageID:     id,
		ChannelID:     "builders",
		FromSessionID: "sess-peer",
		Body:          json.RawMessage(`{"text":"ok"}`),
		Metadata: contract.CoordinationMessageMetadataPayload{
			TaskID:        "task-1",
			RunID:         "run-1",
			ChannelID:     "builders",
			MessageKind:   kind,
			CorrelationID: "run-1",
		},
		Timestamp: time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC),
	}
}
