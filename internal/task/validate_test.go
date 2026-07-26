package task

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

func TestOwnerKindForActor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		actor ActorKind
		want  OwnerKind
	}{
		{"Should map human actor to human owner", ActorKindHuman, OwnerKindHuman},
		{"Should map agent-session actor to agent-session owner", ActorKindAgentSession, OwnerKindAgentSession},
		{"Should map automation actor to automation owner", ActorKindAutomation, OwnerKindAutomation},
		{"Should map extension actor to extension owner", ActorKindExtension, OwnerKindExtension},
		{"Should map network-peer actor to network-peer owner", ActorKindNetworkPeer, OwnerKindNetworkPeer},
		{"Should map an unknown actor kind to an empty owner kind", ActorKind("mystery"), ""},
		{"Should map an empty actor kind to an empty owner kind", ActorKind(""), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := OwnerKindForActor(tc.actor); got != tc.want {
				t.Fatalf("OwnerKindForActor(%q) = %q, want %q", tc.actor, got, tc.want)
			}
		})
	}
}

func TestValidateScopeBinding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		scope       Scope
		workspaceID string
		wantErr     error
	}{
		{name: "global without workspace", scope: ScopeGlobal},
		{name: "workspace with workspace", scope: ScopeWorkspace, workspaceID: "ws-1"},
		{name: "global with workspace", scope: ScopeGlobal, workspaceID: "ws-1", wantErr: ErrInvalidScopeBinding},
		{name: "workspace without workspace", scope: ScopeWorkspace, wantErr: ErrInvalidScopeBinding},
		{name: "unsupported scope", scope: Scope("tenant"), wantErr: ErrValidation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateScopeBinding(tt.scope, tt.workspaceID, "task", "workspace_id")
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("ValidateScopeBinding() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidateScopeBinding() error = nil, want non-nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateScopeBinding() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateImmutableTaskFields(t *testing.T) {
	t.Parallel()

	current := validTask()
	tests := []struct {
		name        string
		mutate      func(*Task)
		wantField   string
		expectError bool
	}{
		{
			name: "created by immutable",
			mutate: func(next *Task) {
				next.CreatedBy.Ref = "human-2"
			},
			wantField:   TaskFieldCreatedBy,
			expectError: true,
		},
		{
			name: "origin immutable",
			mutate: func(next *Task) {
				next.Origin.Ref = "http:api"
			},
			wantField:   TaskFieldOrigin,
			expectError: true,
		},
		{
			name: "scope immutable",
			mutate: func(next *Task) {
				next.Scope = ScopeWorkspace
				next.WorkspaceID = "ws-1"
			},
			wantField:   TaskFieldScope,
			expectError: true,
		},
		{
			name: "workspace id immutable",
			mutate: func(next *Task) {
				next.WorkspaceID = "ws-2"
			},
			wantField:   TaskFieldWorkspaceID,
			expectError: true,
		},
		{
			name: "parent task id immutable",
			mutate: func(next *Task) {
				next.ParentTaskID = "task-2"
			},
			wantField:   TaskFieldParentTaskID,
			expectError: true,
		},
		{
			name: "mutable fields allowed",
			mutate: func(next *Task) {
				next.Title = "Updated"
				next.Description = "changed"
				next.Owner = &Ownership{Kind: OwnerKindPool, Ref: "triage"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			next := current
			tt.mutate(&next)

			err := ValidateImmutableTaskFields(current, next)
			if !tt.expectError {
				if err != nil {
					t.Fatalf("ValidateImmutableTaskFields() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidateImmutableTaskFields() error = nil, want non-nil")
			}
			if !errors.Is(err, ErrImmutableField) {
				t.Fatalf("ValidateImmutableTaskFields() error = %v, want ErrImmutableField", err)
			}
			if !strings.Contains(err.Error(), tt.wantField) {
				t.Fatalf("ValidateImmutableTaskFields() error = %q, want field %q", err.Error(), tt.wantField)
			}
		})
	}
}

func TestValidateImmutableRunFields(t *testing.T) {
	t.Parallel()

	current := validRun()
	current.WorkspaceID = "ws-1"
	current.LoopRunID = "loop-run-1"
	current.PreviousRunID = "run-0"
	current.IdempotencyKey = "idem-1"
	current.DesignationGroupID = "group-1"
	current.SetNetworkState(participation.LocalSpec(), "", "", "")
	tests := []struct {
		name      string
		mutate    func(*Run)
		wantField string
	}{
		{name: "mutable lifecycle fields", mutate: func(next *Run) { next.Status = TaskRunStatusRunning }},
		{name: "task id", mutate: func(next *Run) { next.TaskID = "task-2" }, wantField: runFieldTaskID},
		{name: "workspace id", mutate: func(next *Run) { next.WorkspaceID = "ws-2" }, wantField: runFieldWorkspaceID},
		{name: "attempt", mutate: func(next *Run) { next.Attempt++ }, wantField: runFieldAttempt},
		{name: "run kind", mutate: func(next *Run) { next.RunKind = RunKindCoordinator }, wantField: runFieldRunKind},
		{name: "loop run id", mutate: func(next *Run) { next.LoopRunID = "loop-run-2" }, wantField: runFieldLoopRunID},
		{
			name:      "previous run id",
			mutate:    func(next *Run) { next.PreviousRunID = "run-other" },
			wantField: runFieldPreviousRunID,
		},
		{name: "origin", mutate: func(next *Run) { next.Origin.Ref = "http:api" }, wantField: runFieldOrigin},
		{
			name:      "idempotency key",
			mutate:    func(next *Run) { next.IdempotencyKey = "idem-2" },
			wantField: runFieldIdempotencyKey,
		},
		{
			name: "network spec",
			mutate: func(next *Run) {
				next.SetNetworkState(participation.Spec{
					Version: participation.SpecVersion,
					Mode:    participation.ModeLive,
					Source:  participation.SourceExplicitRequest,
				}, "", "", "")
			},
			wantField: runFieldNetworkSpec,
		},
		{
			name:      "designation group",
			mutate:    func(next *Run) { next.DesignationGroupID = "group-2" },
			wantField: runFieldDesignationGroupID,
		},
		{
			name:      "network wake correlation",
			mutate:    func(next *Run) { next.SetNetworkState(next.NetworkSpecSnapshot(), "wake-1", "sess-1", "owner-1") },
			wantField: runFieldNetworkWake,
		},
	}

	for _, tt := range tests {
		t.Run("Should preserve "+tt.name, func(t *testing.T) {
			t.Parallel()

			next := current
			next.RunNetworkState = cloneRunNetworkStateForImmutabilityTest(current.RunNetworkState)
			tt.mutate(&next)
			err := ValidateImmutableRunFields(current, next)
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("ValidateImmutableRunFields() error = %v", err)
				}
				return
			}
			if !errors.Is(err, ErrImmutableField) || !strings.Contains(err.Error(), tt.wantField) {
				t.Fatalf("ValidateImmutableRunFields() error = %v, want ErrImmutableField for %q", err, tt.wantField)
			}
		})
	}
}

func cloneRunNetworkStateForImmutabilityTest(state *RunNetworkState) *RunNetworkState {
	if state == nil {
		return nil
	}
	cloned := *state
	return &cloned
}

func TestPayloadSizeGuards(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		run     func() error
		wantErr error
	}{
		{
			name: "metadata within limit",
			run: func() error {
				return ValidateMetadataSize(jsonBlob(MaxMetadataBytes-8), "task.metadata")
			},
		},
		{
			name: "metadata over limit",
			run: func() error {
				return ValidateMetadataSize(jsonBlob(MaxMetadataBytes+1), "task.metadata")
			},
			wantErr: ErrPayloadTooLarge,
		},
		{
			name: "payload over limit",
			run: func() error {
				return ValidatePayloadSize(jsonBlob(MaxPayloadBytes+1), "task_event.payload")
			},
			wantErr: ErrPayloadTooLarge,
		},
		{
			name: "result over limit",
			run: func() error {
				return ValidateResultSize(jsonBlob(MaxResultBytes+1), "task_run.result")
			},
			wantErr: ErrPayloadTooLarge,
		},
		{
			name: "invalid json",
			run: func() error {
				return ValidatePayloadSize(json.RawMessage(`{`), "task_event.payload")
			},
			wantErr: ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("payload guard error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("payload guard error = nil, want non-nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("payload guard error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunStatusJSONDecodingShouldRejectUnknownValues(t *testing.T) {
	t.Parallel()

	t.Run("Should decode known task run status", func(t *testing.T) {
		t.Parallel()

		var status RunStatus
		if err := json.Unmarshal([]byte(`"queued"`), &status); err != nil {
			t.Fatalf("json.Unmarshal(task run status) error = %v", err)
		}
		if got, want := status, TaskRunStatusQueued; got != want {
			t.Fatalf("decoded status = %q, want %q", got, want)
		}
	})

	t.Run("Should reject unknown task run status", func(t *testing.T) {
		t.Parallel()

		var status RunStatus
		err := json.Unmarshal([]byte(`"mystery"`), &status)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("json.Unmarshal(task run status) error = %v, want ErrValidation", err)
		}
	})
}

func TestRunKindJSONDecodingShouldRejectUnknownValues(t *testing.T) {
	t.Parallel()

	t.Run("Should decode known task run kind", func(t *testing.T) {
		t.Parallel()

		var kind RunKind
		if err := json.Unmarshal([]byte(`"worker"`), &kind); err != nil {
			t.Fatalf("json.Unmarshal(task run kind) error = %v", err)
		}
		if got, want := kind, RunKindWorker; got != want {
			t.Fatalf("decoded kind = %q, want %q", got, want)
		}
	})

	t.Run("Should reject unknown task run kind", func(t *testing.T) {
		t.Parallel()

		var kind RunKind
		err := json.Unmarshal([]byte(`"sidecar"`), &kind)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("json.Unmarshal(task run kind) error = %v, want ErrValidation", err)
		}
	})
}

func TestRunIsLoopWorkerShouldIdentifyLoopCorrelatedWorkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  Run
		want bool
	}{
		{
			name: "Should reject empty loop run id",
			run:  Run{RunKind: RunKindWorker},
			want: false,
		},
		{
			name: "Should reject coordinator kind",
			run:  Run{RunKind: RunKindCoordinator, LoopRunID: "loop-run-1"},
			want: false,
		},
		{
			name: "Should accept worker with loop run id",
			run:  Run{RunKind: RunKindWorker, LoopRunID: "loop-run-1"},
			want: true,
		},
		{
			name: "Should reject whitespace loop run id",
			run:  Run{RunKind: RunKindWorker, LoopRunID: " \t\n "},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.run.IsLoopWorker(); got != tc.want {
				t.Fatalf("Run.IsLoopWorker() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCoordinatorTerminalShouldValidateShape(t *testing.T) {
	t.Parallel()

	t.Run("Should accept non-empty status", func(t *testing.T) {
		t.Parallel()

		terminal := CoordinatorTerminal{Status: "mystery"}
		if err := terminal.Validate("coordinator.terminal"); err != nil {
			t.Fatalf("CoordinatorTerminal.Validate() error = %v", err)
		}
	})

	t.Run("Should reject empty status", func(t *testing.T) {
		t.Parallel()

		terminal := CoordinatorTerminal{Status: " "}
		err := terminal.Validate("coordinator.terminal")
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("CoordinatorTerminal.Validate() error = %v, want ErrValidation", err)
		}
	})
}

func TestGraphLimitGuards(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		run     func() error
		wantErr error
	}{
		{
			name: "depth at limit",
			run: func() error {
				return ValidateHierarchyDepth(MaxHierarchyDepth)
			},
		},
		{
			name: "depth over limit",
			run: func() error {
				return ValidateHierarchyDepth(MaxHierarchyDepth + 1)
			},
			wantErr: ErrGraphLimitExceeded,
		},
		{
			name: "dependency count over limit",
			run: func() error {
				return ValidateDependencyCount(MaxDependencyCount + 1)
			},
			wantErr: ErrGraphLimitExceeded,
		},
		{
			name: "direct child count over limit",
			run: func() error {
				return ValidateDirectChildCount(MaxDirectChildren + 1)
			},
			wantErr: ErrGraphLimitExceeded,
		},
		{
			name: "negative count rejected",
			run: func() error {
				return ValidateDependencyCount(-1)
			},
			wantErr: ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("graph limit error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("graph limit error = nil, want non-nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("graph limit error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestTaskFieldMutabilityHelpers(t *testing.T) {
	t.Parallel()

	for _, field := range ImmutableTaskFields() {
		if !IsImmutableTaskField(field) {
			t.Fatalf("IsImmutableTaskField(%q) = false, want true", field)
		}
	}
	for _, field := range MutableTaskFields() {
		if !IsMutableTaskField(field) {
			t.Fatalf("IsMutableTaskField(%q) = false, want true", field)
		}
	}
	if IsImmutableTaskField("title") {
		t.Fatal("IsImmutableTaskField(\"title\") = true, want false")
	}
	if IsMutableTaskField("scope") {
		t.Fatal("IsMutableTaskField(\"scope\") = true, want false")
	}
}

func TestDomainValidationHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should task valid", func(t *testing.T) {
		t.Parallel()
		if err := validTask().Validate(); err != nil {
			t.Fatalf("Task.Validate() error = %v", err)
		}
	})

	t.Run("Should draft task valid", func(t *testing.T) {
		t.Parallel()
		taskRecord := validTask()
		taskRecord.Status = TaskStatusDraft
		if err := taskRecord.Validate(); err != nil {
			t.Fatalf("Task.Validate() draft error = %v", err)
		}
	})

	t.Run("Should task invalid owner", func(t *testing.T) {
		t.Parallel()
		taskRecord := validTask()
		taskRecord.Owner = &Ownership{Kind: OwnerKindHuman}
		err := taskRecord.Validate()
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("Task.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should task dependency self dependency", func(t *testing.T) {
		t.Parallel()
		err := (Dependency{
			TaskID:          "task-1",
			DependsOnTaskID: "task-1",
			Kind:            DependencyKindBlocks,
		}).Validate()
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("Dependency.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should task run queued session invalid", func(t *testing.T) {
		t.Parallel()
		run := validRun()
		run.SessionID = "sess-1"
		err := run.Validate()
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("Run.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should task run needs_attention session invalid", func(t *testing.T) {
		t.Parallel()
		run := validRun()
		run.Status = TaskRunStatusNeedsAttention
		run.SessionID = "sess-1"
		err := run.Validate()
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("Run.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should task run needs_attention retain claimed session correlation", func(t *testing.T) {
		t.Parallel()
		run := validRun()
		run.Status = TaskRunStatusNeedsAttention
		run.SessionID = "sess-1"
		run.ClaimedBy = &ActorIdentity{Kind: ActorKindDaemon, Ref: "scheduler"}
		run.ClaimedAt = run.QueuedAt.Add(time.Minute)
		if err := run.Validate(); err != nil {
			t.Fatalf("Run.Validate() error = %v", err)
		}
	})

	t.Run("Should task run lease metadata and capabilities", func(t *testing.T) {
		t.Parallel()

		base := validRun()
		base.Status = TaskRunStatusRunning
		base.ClaimedBy = &ActorIdentity{Kind: ActorKindDaemon, Ref: "scheduler"}
		base.SessionID = "sess-1"
		base.ClaimedAt = base.QueuedAt.Add(time.Minute)
		base.StartedAt = base.ClaimedAt.Add(time.Minute)
		base.ClaimTokenHash = "sha256:" + strings.Repeat("a", 64)
		base.LeaseUntil = base.ClaimedAt.Add(15 * time.Minute)
		base.HeartbeatAt = base.ClaimedAt.Add(30 * time.Second)
		base.RequiredCapabilities = []string{"golang", "sqlite"}
		base.PreferredCapabilities = []string{"claude", "codex"}
		if err := base.Validate(); err != nil {
			t.Fatalf("Run.Validate(valid lease) error = %v", err)
		}

		tests := []struct {
			name   string
			mutate func(*Run)
		}{
			{
				name: "claim token requires hash",
				mutate: func(run *Run) {
					run.ClaimTokenHash = ""
				},
			},
			{
				name: "malformed hash",
				mutate: func(run *Run) {
					run.ClaimTokenHash = "sha256:" + strings.Repeat("A", 64)
				},
			},
			{
				name: "lease before claimed at",
				mutate: func(run *Run) {
					run.LeaseUntil = run.ClaimedAt.Add(-time.Second)
				},
			},
			{
				name: "heartbeat after lease",
				mutate: func(run *Run) {
					run.HeartbeatAt = run.LeaseUntil.Add(time.Second)
				},
			},
			{
				name: "required capability with whitespace",
				mutate: func(run *Run) {
					run.RequiredCapabilities = []string{"golang linux"}
				},
			},
			{
				name: "preferred capability with comma",
				mutate: func(run *Run) {
					run.PreferredCapabilities = []string{"agent,codex"}
				},
			},
			{
				name: "raw token in metadata",
				mutate: func(run *Run) {
					run.Metadata = json.RawMessage(`{"nested":{"claim_token":"raw"}}`)
				},
			},
			{
				name: "raw token in result",
				mutate: func(run *Run) {
					run.Result = json.RawMessage(`{"claim_token":"raw"}`)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				run := base
				tt.mutate(&run)
				err := run.Validate()
				if err == nil || !errors.Is(err, ErrValidation) {
					t.Fatalf("Run.Validate() error = %v, want ErrValidation", err)
				}
			})
		}
	})

	t.Run("Should task event invalid payload", func(t *testing.T) {
		t.Parallel()
		event := validEvent()
		event.Payload = json.RawMessage(`{`)
		err := event.Validate()
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("Event.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should task patch requires mutable field", func(t *testing.T) {
		t.Parallel()
		err := (Patch{}).Validate("patch")
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("Patch.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should run failure requires error", func(t *testing.T) {
		t.Parallel()
		err := (RunFailure{}).Validate("failure")
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("RunFailure.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should start task session requires matching task and run", func(t *testing.T) {
		t.Parallel()
		req := StartTaskSession{
			Task:  validTask(),
			Run:   validRun(),
			Actor: validActorContext(),
		}
		req.Run.TaskID = "task-2"
		err := req.Validate()
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("StartTaskSession.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should task query validates filters", func(t *testing.T) {
		t.Parallel()
		err := (Query{
			Scope:         ScopeWorkspace,
			WorkspaceID:   "ws-1",
			Status:        TaskStatusReady,
			Priority:      PriorityHigh,
			ApprovalState: ApprovalStatePending,
			OwnerKind:     OwnerKindPool,
			Limit:         10,
		}).Validate("query")
		if err != nil {
			t.Fatalf("Query.Validate() error = %v", err)
		}
	})

	t.Run("Should start task session valid", func(t *testing.T) {
		t.Parallel()
		req := StartTaskSession{
			Task:  validTask(),
			Run:   validRun(),
			Actor: validActorContext(),
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("StartTaskSession.Validate() error = %v", err)
		}
	})
}

func TestTaskSemanticValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		run     func() error
		wantErr error
	}{
		{
			name: "draft task cannot be closed",
			run: func() error {
				taskRecord := validTask()
				taskRecord.Status = TaskStatusDraft
				taskRecord.ClosedAt = time.Date(2026, 4, 14, 12, 45, 0, 0, time.UTC)
				return taskRecord.Validate()
			},
			wantErr: ErrValidation,
		},
		{
			name: "task invalid priority",
			run: func() error {
				taskRecord := validTask()
				taskRecord.Priority = Priority("p0")
				return taskRecord.Validate()
			},
			wantErr: ErrValidation,
		},
		{
			name: "task invalid max attempts",
			run: func() error {
				taskRecord := validTask()
				taskRecord.MaxAttempts = -1
				return taskRecord.Validate()
			},
			wantErr: ErrValidation,
		},
		{
			name: "task manual approval pending valid",
			run: func() error {
				taskRecord := validTask()
				taskRecord.ApprovalPolicy = ApprovalPolicyManual
				taskRecord.ApprovalState = ApprovalStatePending
				return taskRecord.Validate()
			},
		},
		{
			name: "task no-approval with pending state invalid",
			run: func() error {
				taskRecord := validTask()
				taskRecord.ApprovalState = ApprovalStatePending
				return taskRecord.Validate()
			},
			wantErr: ErrValidation,
		},
		{
			name: "task manual approval with not-required state invalid",
			run: func() error {
				taskRecord := validTask()
				taskRecord.ApprovalPolicy = ApprovalPolicyManual
				taskRecord.ApprovalState = ApprovalStateNotRequired
				return taskRecord.Validate()
			},
			wantErr: ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("semantic validation error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("semantic validation error = nil, want non-nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("semantic validation error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func validTask() Task {
	now := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)
	return Task{
		ID:             "task-1",
		Identifier:     "TASK-1",
		Scope:          ScopeGlobal,
		Title:          "Bootstrap internal/task",
		Description:    "Create the task domain",
		Priority:       PriorityHigh,
		MaxAttempts:    DefaultTaskMaxAttempts,
		Status:         TaskStatusReady,
		ApprovalPolicy: ApprovalPolicyNone,
		ApprovalState:  ApprovalStateNotRequired,
		Owner:          &Ownership{Kind: OwnerKindHuman, Ref: "user-1"},
		CreatedBy:      ActorIdentity{Kind: ActorKindHuman, Ref: "user-1"},
		Origin:         Origin{Kind: OriginKindCLI, Ref: "agh task create"},
		CreatedAt:      now,
		UpdatedAt:      now,
		Metadata:       json.RawMessage(`{"source":"cli"}`),
		ClosedAt:       time.Time{},
		ParentTaskID:   "",
	}
}

func validRun() Run {
	now := time.Date(2026, 4, 14, 12, 30, 0, 0, time.UTC)
	return Run{
		ID:       "run-1",
		TaskID:   "task-1",
		Status:   TaskRunStatusQueued,
		Attempt:  1,
		Origin:   Origin{Kind: OriginKindCLI, Ref: "agh task run enqueue"},
		QueuedAt: now,
		Result:   json.RawMessage(`{"ok":true}`),
	}
}

func validEvent() Event {
	now := time.Date(2026, 4, 14, 13, 0, 0, 0, time.UTC)
	return Event{
		ID:        "evt-1",
		TaskID:    "task-1",
		EventType: "task.created",
		Actor:     ActorIdentity{Kind: ActorKindHuman, Ref: "user-1"},
		Origin:    Origin{Kind: OriginKindCLI, Ref: "agh task create"},
		Payload:   json.RawMessage(`{"source":"cli"}`),
		Timestamp: now,
	}
}

func validTaskRunIdempotency() RunIdempotency {
	now := time.Date(2026, 4, 14, 13, 30, 0, 0, time.UTC)
	return RunIdempotency{
		IdempotencyKey: "idem-1",
		RunID:          "run-1",
		Origin:         Origin{Kind: OriginKindAutomation, Ref: "rule:nightly"},
		CreatedAt:      now,
	}
}

func validActorContext() ActorContext {
	return ActorContext{
		Actor:  ActorIdentity{Kind: ActorKindHuman, Ref: "user-1"},
		Origin: Origin{Kind: OriginKindCLI, Ref: "agh task run start"},
		Scope:  CallerScope{Operator: true},
		Authority: Authority{
			Read:            true,
			Write:           true,
			CreateGlobal:    true,
			CreateWorkspace: true,
		},
	}
}

func jsonBlob(targetSize int) json.RawMessage {
	if targetSize <= 2 {
		return json.RawMessage(`""`)
	}
	return json.RawMessage(`"` + strings.Repeat("a", targetSize-2) + `"`)
}

func TestEnumAndIdentityValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		run     func() error
		wantErr error
	}{
		{name: "task status valid", run: func() error { return TaskStatusReady.Validate("status") }},
		{name: "task status draft valid", run: func() error { return TaskStatusDraft.Validate("status") }},
		{
			name: "Should accept needs_attention task status",
			run:  func() error { return TaskStatusNeedsAttention.Validate("status") },
		},
		{
			name:    "task status invalid",
			run:     func() error { return Status("waiting").Validate("status") },
			wantErr: ErrValidation,
		},
		{name: "task priority valid", run: func() error { return PriorityHigh.Validate("task.priority") }},
		{
			name:    "task priority invalid",
			run:     func() error { return Priority("rush").Validate("task.priority") },
			wantErr: ErrValidation,
		},
		{
			name: "approval policy valid",
			run:  func() error { return ApprovalPolicyManual.Validate("task.approval_policy") },
		},
		{
			name:    "approval policy invalid",
			run:     func() error { return ApprovalPolicy("auto").Validate("task.approval_policy") },
			wantErr: ErrValidation,
		},
		{
			name: "approval state valid",
			run:  func() error { return ApprovalStatePending.Validate("task.approval_state") },
		},
		{
			name:    "approval state invalid",
			run:     func() error { return ApprovalState("queued").Validate("task.approval_state") },
			wantErr: ErrValidation,
		},
		{name: "task run status valid", run: func() error { return TaskRunStatusRunning.Validate("run.status") }},
		{
			name: "Should accept needs_attention run status",
			run:  func() error { return TaskRunStatusNeedsAttention.Validate("run.status") },
		},
		{
			name:    "Should reject unsupported task run status",
			run:     func() error { return ParseRunStatus("paused").Validate("run.status") },
			wantErr: ErrValidation,
		},
		{name: "actor kind valid", run: func() error { return ActorKindHuman.Validate("actor.kind") }},
		{
			name:    "actor kind invalid",
			run:     func() error { return ActorKind("bot").Validate("actor.kind") },
			wantErr: ErrValidation,
		},
		{name: "owner kind valid", run: func() error { return OwnerKindPool.Validate("owner.kind") }},
		{
			name:    "owner kind invalid",
			run:     func() error { return OwnerKind("queue").Validate("owner.kind") },
			wantErr: ErrValidation,
		},
		{name: "origin kind valid", run: func() error { return OriginKindCLI.Validate("origin.kind") }},
		{
			name:    "origin kind invalid",
			run:     func() error { return OriginKind("mqtt").Validate("origin.kind") },
			wantErr: ErrValidation,
		},
		{name: "dependency kind valid", run: func() error { return DependencyKindBlocks.Validate("dependency.kind") }},
		{
			name:    "dependency kind invalid",
			run:     func() error { return DependencyKind("soft").Validate("dependency.kind") },
			wantErr: ErrValidation,
		},
		{name: "stop reason valid", run: func() error { return StopReasonCancellation.Validate("stop.reason") }},
		{
			name:    "stop reason invalid",
			run:     func() error { return StopReason("later").Validate("stop.reason") },
			wantErr: ErrValidation,
		},
		{
			name: "run boot recovery action valid",
			run:  func() error { return RunBootRecoveryMarkRunning.Validate("recovery.action") },
		},
		{
			name:    "run boot recovery action invalid",
			run:     func() error { return RunBootRecoveryAction("resume").Validate("recovery.action") },
			wantErr: ErrValidation,
		},
		{name: "actor identity valid", run: func() error { return validTask().CreatedBy.Validate("actor") }},
		{
			name:    "actor identity invalid",
			run:     func() error { return ActorIdentity{Kind: ActorKindHuman}.Validate("actor") },
			wantErr: ErrValidation,
		},
		{name: "origin valid", run: func() error { return validTask().Origin.Validate("origin") }},
		{
			name:    "origin invalid",
			run:     func() error { return Origin{Kind: OriginKindCLI}.Validate("origin") },
			wantErr: ErrValidation,
		},
		{name: "authority valid", run: func() error { return validActorContext().Authority.Validate("authority") }},
		{name: "authority invalid", run: func() error {
			return Authority{CreateGlobal: true}.Validate("authority")
		}, wantErr: ErrValidation},
		{name: "actor context valid", run: func() error { return validActorContext().Validate() }},
		{name: "actor context invalid", run: func() error {
			ctx := validActorContext()
			ctx.Actor.Ref = ""
			return ctx.Validate()
		}, wantErr: ErrValidation},
		{name: "run boot recovery valid", run: func() error {
			return RunBootRecovery{Action: RunBootRecoveryFail}.Validate("recovery")
		}},
		{name: "run boot recovery invalid", run: func() error {
			return RunBootRecovery{}.Validate("recovery")
		}, wantErr: ErrValidation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("validation error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("validation error = nil, want non-nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("validation error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequestAndQueryValidation(t *testing.T) {
	t.Parallel()

	title := "Updated title"
	metadata := json.RawMessage(`{"source":"web"}`)
	priority := PriorityUrgent
	maxAttempts := 5
	approvalPolicy := ApprovalPolicyManual

	tests := []struct {
		name    string
		run     func() error
		wantErr error
	}{
		{
			name: "create task valid",
			run: func() error {
				return CreateTask{
					Scope:          ScopeWorkspace,
					Title:          "Create task",
					Priority:       PriorityHigh,
					MaxAttempts:    new(4),
					ApprovalPolicy: ApprovalPolicyManual,
					Owner:          &Ownership{Kind: OwnerKindPool, Ref: "triage"},
					Metadata:       json.RawMessage(`{"kind":"bootstrap"}`),
					WorkspaceID:    "ws-1",
				}.Validate("create")
			},
		},
		{
			name: "create task invalid max attempts zero",
			run: func() error {
				return CreateTask{
					Scope:       ScopeGlobal,
					Title:       "Create task",
					MaxAttempts: new(0),
				}.Validate("create")
			},
			wantErr: ErrValidation,
		},
		{
			name: "create task invalid parent self",
			run: func() error {
				return CreateTask{
					ID:           "task-1",
					Scope:        ScopeGlobal,
					Title:        "Create task",
					ParentTaskID: "task-1",
				}.Validate("create")
			},
			wantErr: ErrValidation,
		},
		{
			name: "task patch valid",
			run: func() error {
				return Patch{
					Title:          &title,
					Priority:       &priority,
					MaxAttempts:    &maxAttempts,
					ApprovalPolicy: &approvalPolicy,
					Metadata:       &metadata,
				}.Validate("patch")
			},
		},
		{
			name: "task patch invalid max attempts",
			run: func() error {
				zero := 0
				return Patch{MaxAttempts: &zero}.Validate("patch")
			},
			wantErr: ErrValidation,
		},
		{
			name: "task patch invalid priority",
			run: func() error {
				invalidPriority := Priority("rush")
				return Patch{Priority: &invalidPriority}.Validate("patch")
			},
			wantErr: ErrValidation,
		},
		{
			name: "task patch owner conflict",
			run: func() error {
				return Patch{
					Owner:      &Ownership{Kind: OwnerKindPool, Ref: "triage"},
					ClearOwner: true,
				}.Validate("patch")
			},
			wantErr: ErrValidation,
		},
		{
			name: "cancel task metadata valid",
			run: func() error {
				return CancelTask{Metadata: json.RawMessage(`{"reason":"user"}`)}.Validate("cancel")
			},
		},
		{
			name: "add dependency valid",
			run: func() error {
				return AddDependency{
					TaskID:          "task-1",
					DependsOnTaskID: "task-0",
					Kind:            DependencyKindBlocks,
				}.Validate("dependency")
			},
		},
		{
			name: "add dependency invalid",
			run: func() error {
				return AddDependency{
					TaskID:          "task-1",
					DependsOnTaskID: "task-1",
					Kind:            DependencyKindBlocks,
				}.Validate("dependency")
			},
			wantErr: ErrValidation,
		},
		{
			name: "enqueue run valid",
			run: func() error {
				return EnqueueRun{TaskID: "task-1"}.Validate("enqueue")
			},
		},
		{
			name: "enqueue run invalid",
			run: func() error {
				return EnqueueRun{}.Validate("enqueue")
			},
			wantErr: ErrValidation,
		},
		{
			name: "Should reject unsupported enqueue run kind",
			run: func() error {
				return EnqueueRun{
					TaskID:  "task-1",
					RunKind: ParseRunKind("agent"),
				}.Validate("enqueue")
			},
			wantErr: ErrValidation,
		},
		{
			name: "Should reject unsupported queue reservation run kind",
			run: func() error {
				return QueueRunReservation{
					TaskID:  "task-1",
					RunID:   "run-1",
					RunKind: ParseRunKind("agent"),
					Origin:  Origin{Kind: OriginKindDaemon, Ref: "loop"},
				}.Validate("reservation")
			},
			wantErr: ErrValidation,
		},
		{
			name: "start run valid",
			run: func() error {
				return StartRun{}.Validate("start")
			},
		},
		{
			name: "start run invalid path",
			run: func() error {
				return StartRun{}.Validate("")
			},
			wantErr: ErrValidation,
		},
		{
			name: "cancel run metadata valid",
			run: func() error {
				return CancelRun{Metadata: json.RawMessage(`{"reason":"user"}`)}.Validate("cancel")
			},
		},
		{
			name: "run result valid",
			run: func() error {
				return RunResult{Value: json.RawMessage(`{"ok":true}`)}.Validate("result")
			},
		},
		{
			name: "run result rejects raw claim token",
			run: func() error {
				return RunResult{Value: json.RawMessage(`{"claim_token":"secret"}`)}.Validate("result")
			},
			wantErr: ErrValidation,
		},
		{
			name: "run failure rejects raw claim token metadata",
			run: func() error {
				return RunFailure{
					Error:    "boom",
					Metadata: json.RawMessage(`{"nested":{"claim_token":"secret"}}`),
				}.Validate("failure")
			},
			wantErr: ErrValidation,
		},
		{
			name: "task run query valid",
			run: func() error {
				return RunQuery{
					Status:               TaskRunStatusRunning,
					ParticipationChannel: "builders",
					Limit:                2,
				}.Validate("runs")
			},
		},
		{
			name: "Should reject invalid task run participation channel",
			run: func() error {
				return RunQuery{ParticipationChannel: "invalid channel"}.Validate("runs")
			},
			wantErr: ErrValidation,
		},
		{
			name: "Should reject unsupported task run query status",
			run: func() error {
				return RunQuery{Status: ParseRunStatus("paused")}.Validate("runs")
			},
			wantErr: ErrValidation,
		},
		{
			name: "task run query invalid",
			run: func() error {
				return RunQuery{Limit: -1}.Validate("runs")
			},
			wantErr: ErrValidation,
		},
		{
			name: "task event query valid",
			run: func() error {
				return EventQuery{Limit: 1}.Validate("events")
			},
		},
		{
			name: "task event query invalid",
			run: func() error {
				return EventQuery{Limit: -1}.Validate("events")
			},
			wantErr: ErrValidation,
		},
		{
			name: "task run idempotency valid",
			run: func() error {
				return validTaskRunIdempotency().Validate()
			},
		},
		{
			name: "task run idempotency invalid",
			run: func() error {
				record := validTaskRunIdempotency()
				record.Origin.Ref = ""
				return record.Validate()
			},
			wantErr: ErrValidation,
		},
		{
			name: "session ref valid",
			run: func() error {
				return SessionRef{SessionID: "sess-1"}.Validate()
			},
		},
		{
			name: "session ref invalid",
			run: func() error {
				return SessionRef{}.Validate()
			},
			wantErr: ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("validation error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("validation error = nil, want non-nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("validation error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestOwnershipIsZero(t *testing.T) {
	t.Parallel()

	if !(Ownership{}).IsZero() {
		t.Fatal("Ownership{}.IsZero() = false, want true")
	}
	if (Ownership{Kind: OwnerKindPool, Ref: "triage"}).IsZero() {
		t.Fatal("Ownership{pool}.IsZero() = true, want false")
	}
}

func TestAdditionalBranchCoverage(t *testing.T) {
	t.Parallel()

	t.Run("Should task missing id", func(t *testing.T) {
		t.Parallel()
		taskRecord := validTask()
		taskRecord.ID = ""
		err := taskRecord.Validate()
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("Task.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should task parent self", func(t *testing.T) {
		t.Parallel()
		taskRecord := validTask()
		taskRecord.ParentTaskID = taskRecord.ID
		err := taskRecord.Validate()
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("Task.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should task missing title", func(t *testing.T) {
		t.Parallel()
		taskRecord := validTask()
		taskRecord.Title = ""
		err := taskRecord.Validate()
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("Task.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should task run missing claimed by ref", func(t *testing.T) {
		t.Parallel()
		run := validRun()
		run.Status = TaskRunStatusClaimed
		run.ClaimedBy = &ActorIdentity{Kind: ActorKindHuman}
		err := run.Validate()
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("Run.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should task run invalid attempt", func(t *testing.T) {
		t.Parallel()
		run := validRun()
		run.Attempt = 0
		err := run.Validate()
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("Run.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should task event missing event type", func(t *testing.T) {
		t.Parallel()
		event := validEvent()
		event.EventType = ""
		err := event.Validate()
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("Event.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should task event missing origin", func(t *testing.T) {
		t.Parallel()
		event := validEvent()
		event.Origin.Ref = ""
		err := event.Validate()
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("Event.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should require task scope unless an internal all-task replay is explicit", func(t *testing.T) {
		t.Parallel()

		if err := (EventRecordQuery{}).Validate("task_event_record_query"); !errors.Is(err, ErrValidation) {
			t.Fatalf("EventRecordQuery(empty).Validate() error = %v, want %v", err, ErrValidation)
		}
		if err := (EventRecordQuery{AllTasks: true}).Validate("task_event_record_query"); err != nil {
			t.Fatalf("EventRecordQuery(AllTasks).Validate() error = %v", err)
		}
		if err := (EventRecordQuery{TaskID: "task-1", AllTasks: true}).Validate(
			"task_event_record_query",
		); !errors.Is(
			err,
			ErrValidation,
		) {
			t.Fatalf("EventRecordQuery(conflicting scope).Validate() error = %v, want %v", err, ErrValidation)
		}
	})

	t.Run("Should start task session invalid actor", func(t *testing.T) {
		t.Parallel()
		req := StartTaskSession{
			Task:  validTask(),
			Run:   validRun(),
			Actor: validActorContext(),
		}
		req.Actor.Authority = Authority{CreateGlobal: true}
		err := req.Validate()
		if err == nil || !errors.Is(err, ErrValidation) {
			t.Fatalf("StartTaskSession.Validate() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should nested path helper empty path", func(t *testing.T) {
		t.Parallel()
		if got := nestedPath("", "field"); got != "field" {
			t.Fatalf("nestedPath('', 'field') = %q, want field", got)
		}
		if got := nestedPath("root", ""); got != "root" {
			t.Fatalf("nestedPath('root', '') = %q, want root", got)
		}
	})
}
