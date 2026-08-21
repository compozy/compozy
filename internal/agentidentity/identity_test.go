package agentidentity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func TestResolveValidatesAgentCallerIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	active := SessionSnapshot{
		ID:            "sess-1",
		ProfileID:     "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Name:          "worker",
		AgentName:     "coder",
		Provider:      "test-provider",
		WorkspaceID:   "ws-1",
		WorkspacePath: "/workspace",
		State:         session.StateActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	tests := []struct {
		name              string
		credentials       Credentials
		session           SessionSnapshot
		lookupErr         error
		expectedWorkspace string
		originKind        taskpkg.OriginKind
		wantErr           error
		wantExit          int
		wantOrigin        taskpkg.OriginKind
	}{
		{
			name: "Should reject missing session id",
			credentials: Credentials{
				AgentName: "coder",
			},
			wantErr:  ErrIdentityRequired,
			wantExit: ExitIdentityRequired,
		},
		{
			name: "Should reject missing agent name",
			credentials: Credentials{
				SessionID: "sess-1",
			},
			wantErr:  ErrIdentityRequired,
			wantExit: ExitIdentityRequired,
		},
		{
			name: "Should reject unknown session",
			credentials: Credentials{
				SessionID: "missing",
				AgentName: "coder",
			},
			lookupErr: session.ErrSessionNotFound,
			wantErr:   ErrIdentityStale,
			wantExit:  ExitIdentityInvalid,
		},
		{
			name: "Should reject stopped session",
			credentials: Credentials{
				SessionID: "sess-1",
				AgentName: "coder",
			},
			session: func() SessionSnapshot {
				s := active
				s.State = session.StateStopped
				return s
			}(),
			wantErr:  ErrIdentityStale,
			wantExit: ExitIdentityInvalid,
		},
		{
			name: "Should reject agent mismatch",
			credentials: Credentials{
				SessionID: "sess-1",
				AgentName: "reviewer",
			},
			session:  active,
			wantErr:  ErrIdentityMismatch,
			wantExit: ExitIdentityInvalid,
		},
		{
			name: "Should reject workspace mismatch",
			credentials: Credentials{
				SessionID: "sess-1",
				AgentName: "coder",
			},
			session:           active,
			expectedWorkspace: "ws-2",
			wantErr:           ErrIdentityUnauthorized,
			wantExit:          ExitUnauthorized,
		},
		{
			name: "Should accept valid cli identity",
			credentials: Credentials{
				SessionID: " sess-1 ",
				AgentName: " coder ",
			},
			session:    active,
			originKind: taskpkg.OriginKindCLI,
			wantOrigin: taskpkg.OriginKindCLI,
		},
		{
			name: "Should accept valid uds identity",
			credentials: Credentials{
				SessionID: "sess-1",
				AgentName: "coder",
			},
			session:    active,
			originKind: taskpkg.OriginKindUDS,
			wantOrigin: taskpkg.OriginKindUDS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lookup := func(_ context.Context, sessionID string) (SessionSnapshot, error) {
				if tt.lookupErr != nil {
					return SessionSnapshot{}, tt.lookupErr
				}
				if tt.session.ID == "" {
					return active, nil
				}
				if strings.TrimSpace(sessionID) != tt.session.ID {
					t.Fatalf("lookup sessionID = %q, want %q", sessionID, tt.session.ID)
				}
				return tt.session, nil
			}

			caller, err := Resolve(context.Background(), ResolveOptions{
				Credentials:         tt.credentials,
				Lookup:              lookup,
				ExpectedWorkspaceID: tt.expectedWorkspace,
				OriginKind:          tt.originKind,
				OriginRef:           "agent.test",
			})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Resolve() error = %v, want %v", err, tt.wantErr)
				}
				if got := ExitCodeForError(err); got != tt.wantExit {
					t.Fatalf("ExitCodeForError() = %d, want %d", got, tt.wantExit)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if caller.Session.ID != "sess-1" || caller.Session.AgentName != "coder" {
				t.Fatalf("caller.Session = %#v, want validated session", caller.Session)
			}
			if caller.Session.ProfileID != active.ProfileID {
				t.Fatalf("caller.Session.ProfileID = %q, want %q", caller.Session.ProfileID, active.ProfileID)
			}
			if caller.Actor.Actor.Kind != taskpkg.ActorKindAgentSession || caller.Actor.Actor.Ref != "sess-1" {
				t.Fatalf("caller.Actor.Actor = %#v, want agent session sess-1", caller.Actor.Actor)
			}
			if got, want := caller.Actor.Scope, (taskpkg.CallerScope{
				SessionID:   "sess-1",
				WorkspaceID: "ws-1",
			}); got != want {
				t.Fatalf("caller.Actor.Scope = %#v, want %#v", got, want)
			}
			if caller.Actor.Origin.Kind != tt.wantOrigin || caller.Actor.Origin.Ref != "agent.test" {
				t.Fatalf("caller.Actor.Origin = %#v, want %s agent.test", caller.Actor.Origin, tt.wantOrigin)
			}
		})
	}
}

func TestResolveAuthorizesWorkspaceAccess(t *testing.T) {
	t.Parallel()

	snapshot := SessionSnapshot{
		ID:          "sess-1",
		ProfileID:   store.DefaultProfileID,
		AgentName:   "coder",
		WorkspaceID: "ws-home",
		State:       session.StateActive,
	}
	lookup := func(context.Context, string) (SessionSnapshot, error) { return snapshot, nil }
	credentials := Credentials{SessionID: snapshot.ID, AgentName: snapshot.AgentName}

	t.Run("Should preserve the validated home workspace when policy allows", func(t *testing.T) {
		t.Parallel()

		var received workspaceaccess.Request
		caller, err := Resolve(t.Context(), ResolveOptions{
			Credentials:         credentials,
			Lookup:              lookup,
			ExpectedWorkspaceID: "ws-target",
			WorkspaceAccess: workspaceAccessPolicyFunc(func(
				_ context.Context,
				req workspaceaccess.Request,
			) (workspaceaccess.Decision, error) {
				received = req
				return workspaceaccess.Decision{Allowed: true}, nil
			}),
		})
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if caller.Session.WorkspaceID != snapshot.WorkspaceID {
			t.Fatalf("caller workspace = %q, want home %q", caller.Session.WorkspaceID, snapshot.WorkspaceID)
		}
		if received.Actor.Kind != workspaceaccess.ActorAgentSession ||
			received.Actor.SessionID != snapshot.ID ||
			received.TargetWorkspaceID != "ws-target" ||
			received.Seam != workspaceaccess.SeamIdentity {
			t.Fatalf("Authorize() request = %#v, want validated identity seam", received)
		}
	})

	t.Run("Should convert a prompt eligible denial into identity unauthorized", func(t *testing.T) {
		t.Parallel()

		_, err := Resolve(t.Context(), ResolveOptions{
			Credentials:         credentials,
			Lookup:              lookup,
			ExpectedWorkspaceID: "ws-target",
			WorkspaceAccess: workspaceAccessPolicyFunc(func(
				context.Context,
				workspaceaccess.Request,
			) (workspaceaccess.Decision, error) {
				return workspaceaccess.Decision{PromptEligible: true}, nil
			}),
		})
		if !errors.Is(err, ErrIdentityUnauthorized) {
			t.Fatalf("Resolve() error = %v, want ErrIdentityUnauthorized", err)
		}
		if ExitCodeForError(err) != ExitUnauthorized {
			t.Fatalf("ExitCodeForError() = %d, want %d", ExitCodeForError(err), ExitUnauthorized)
		}
		if !strings.Contains(err.Error(), workspaceaccess.DenialHint) {
			t.Fatalf("Resolve() error = %q, want denial hint", err)
		}
	})

	t.Run("Should preserve policy failures while denying workspace access", func(t *testing.T) {
		t.Parallel()

		policyErr := errors.New("policy store unavailable")
		_, err := Resolve(t.Context(), ResolveOptions{
			Credentials:         credentials,
			Lookup:              lookup,
			ExpectedWorkspaceID: "ws-target",
			WorkspaceAccess: workspaceAccessPolicyFunc(func(
				context.Context,
				workspaceaccess.Request,
			) (workspaceaccess.Decision, error) {
				return workspaceaccess.Decision{}, policyErr
			}),
		})
		if !errors.Is(err, ErrIdentityUnauthorized) || !errors.Is(err, policyErr) {
			t.Fatalf("Resolve() error = %v, want identity denial joined with policy failure", err)
		}
	})
}

type workspaceAccessPolicyFunc func(
	context.Context,
	workspaceaccess.Request,
) (workspaceaccess.Decision, error)

func (f workspaceAccessPolicyFunc) Authorize(
	ctx context.Context,
	req workspaceaccess.Request,
) (workspaceaccess.Decision, error) {
	return f(ctx, req)
}

func TestIdentityErrorDiagnosticItem(t *testing.T) {
	t.Parallel()

	t.Run("Should expose redacted DiagnosticItem", func(t *testing.T) {
		t.Parallel()

		err := identityError(
			ErrIdentityRequired,
			contract.CodeIdentityRequired,
			"agent token=identity-secret is required",
			"export COMPOZY_SESSION_ID",
		)

		identityErr, identityErrMatched := errors.AsType[*Error](err)
		if !identityErrMatched {
			t.Fatal("errors.As() = false, want *Error")
		}
		item := identityErr.ToDiagnosticItem()
		if item.Code != contract.CodeIdentityRequired {
			t.Fatalf("DiagnosticItem.Code = %q, want %q", item.Code, contract.CodeIdentityRequired)
		}
		if item.SuggestedCommand != "export COMPOZY_SESSION_ID" {
			t.Fatalf("DiagnosticItem.SuggestedCommand = %q, want action", item.SuggestedCommand)
		}
		if strings.Contains(item.Message, "identity-secret") {
			t.Fatalf("DiagnosticItem.Message = %q leaked secret", item.Message)
		}
	})
}

func TestErrorOutputConventionsRenderStableJSONAndJSONL(t *testing.T) {
	t.Parallel()

	err := &Error{
		Code:    "identity_required",
		Message: EnvSessionID + " is required for agent commands",
		Action:  "run this command from a CompozyOS-managed agent session",
		Err:     ErrIdentityRequired,
	}

	t.Run("Should render stable JSON error payload", func(t *testing.T) {
		t.Parallel()

		jsonPayload, jsonErr := MarshalErrorJSON(err)
		if jsonErr != nil {
			t.Fatalf("MarshalErrorJSON() error = %v", jsonErr)
		}
		var jsonObject struct {
			Error ErrorPayload `json:"error"`
		}
		if unmarshalErr := json.Unmarshal(jsonPayload, &jsonObject); unmarshalErr != nil {
			t.Fatalf("json.Unmarshal(JSON) error = %v", unmarshalErr)
		}
		if jsonObject.Error.Code != "identity_required" || jsonObject.Error.ExitCode != ExitIdentityRequired {
			t.Fatalf("JSON error = %#v, want stable identity error payload", jsonObject.Error)
		}
	})

	t.Run("Should render stable JSONL error frame", func(t *testing.T) {
		t.Parallel()

		jsonlPayload, jsonlErr := MarshalErrorJSONL(err)
		if jsonlErr != nil {
			t.Fatalf("MarshalErrorJSONL() error = %v", jsonlErr)
		}
		if len(jsonlPayload) == 0 || jsonlPayload[len(jsonlPayload)-1] != '\n' {
			t.Fatalf("JSONL payload missing trailing newline: %q", jsonlPayload)
		}
		var jsonlObject struct {
			Type  string       `json:"type"`
			Error ErrorPayload `json:"error"`
		}
		jsonlFrame := []byte(strings.TrimSuffix(string(jsonlPayload), "\n"))
		if unmarshalErr := json.Unmarshal(jsonlFrame, &jsonlObject); unmarshalErr != nil {
			t.Fatalf("json.Unmarshal(JSONL) error = %v", unmarshalErr)
		}
		if jsonlObject.Type != "error" || jsonlObject.Error.Action == "" {
			t.Fatalf("JSONL object = %#v, want error frame with actionable payload", jsonlObject)
		}
	})
}

func TestResolveRejectsUnavailableAndMalformedLookupResults(t *testing.T) {
	t.Parallel()

	creds := Credentials{
		SessionID: "sess-1",
		AgentName: "coder",
	}

	tests := []struct {
		name     string
		ctx      context.Context
		lookup   SessionLookup
		wantErr  error
		wantExit int
	}{
		{
			name: "Should reject nil context",
			lookup: func(_ context.Context, _ string) (SessionSnapshot, error) {
				return SessionSnapshot{
					ID:        "sess-1",
					ProfileID: store.DefaultProfileID,
					AgentName: "coder",
					State:     session.StateActive,
				}, nil
			},
			wantErr: ErrIdentityLookupUnavailable,
		},
		{
			name:    "Should reject nil lookup",
			ctx:     context.Background(),
			wantErr: ErrIdentityLookupUnavailable,
		},
		{
			name: "Should reject empty returned session id",
			ctx:  context.Background(),
			lookup: func(_ context.Context, _ string) (SessionSnapshot, error) {
				return SessionSnapshot{
					ProfileID: store.DefaultProfileID,
					AgentName: "coder",
					State:     session.StateActive,
				}, nil
			},
			wantErr: ErrIdentityStale,
		},
		{
			name: "Should reject different returned session id",
			ctx:  context.Background(),
			lookup: func(_ context.Context, _ string) (SessionSnapshot, error) {
				return SessionSnapshot{
					ID:        "sess-2",
					ProfileID: store.DefaultProfileID,
					AgentName: "coder",
					State:     session.StateActive,
				}, nil
			},
			wantErr: ErrIdentityMismatch,
		},
		{
			name: "Should reject empty returned profile id",
			ctx:  context.Background(),
			lookup: func(_ context.Context, _ string) (SessionSnapshot, error) {
				return SessionSnapshot{
					ID:        "sess-1",
					AgentName: "coder",
					State:     session.StateActive,
				}, nil
			},
			wantErr: ErrIdentityStale,
		},
		{
			name: "Should classify backend lookup failures as unavailable",
			ctx:  context.Background(),
			lookup: func(_ context.Context, _ string) (SessionSnapshot, error) {
				return SessionSnapshot{}, fmt.Errorf("read session metadata: %w", os.ErrPermission)
			},
			wantErr:  ErrIdentityLookupUnavailable,
			wantExit: ExitUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Resolve(tt.ctx, ResolveOptions{
				Credentials: creds,
				Lookup:      tt.lookup,
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Resolve() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantExit != 0 {
				if got := ExitCodeForError(err); got != tt.wantExit {
					t.Fatalf("ExitCodeForError() = %d, want %d", got, tt.wantExit)
				}
			}
		})
	}
}

func TestResolveDefaultsAgentSessionOrigin(t *testing.T) {
	t.Parallel()

	t.Run("Should default to agent session origin", func(t *testing.T) {
		t.Parallel()

		caller, err := Resolve(context.Background(), ResolveOptions{
			Credentials: Credentials{
				SessionID: "sess-1",
				AgentName: "coder",
			},
			Lookup: func(_ context.Context, _ string) (SessionSnapshot, error) {
				return SessionSnapshot{
					ID:          " sess-1 ",
					ProfileID:   store.DefaultProfileID,
					AgentName:   " coder ",
					WorkspaceID: " ws-1 ",
					State:       session.StateActive,
				}, nil
			},
		})
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if caller.Actor.Origin.Kind != taskpkg.OriginKindAgentSession || caller.Actor.Origin.Ref != "agent.session" {
			t.Fatalf("caller.Actor.Origin = %#v, want default agent_session origin", caller.Actor.Origin)
		}
	})
}

func TestSessionSnapshotFromInfo(t *testing.T) {
	t.Parallel()

	t.Run("Should return an empty snapshot for nil info", func(t *testing.T) {
		t.Parallel()

		if got := SessionSnapshotFromInfo(nil); got != (SessionSnapshot{}) {
			t.Fatalf("SessionSnapshotFromInfo(nil) = %#v, want empty snapshot", got)
		}
	})

	t.Run("Should copy fields from session info", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 26, 11, 0, 0, 0, time.UTC)
		wantParticipation := participation.Spec{
			Version:         participation.SpecVersion,
			Mode:            participation.ModeLive,
			WorkspaceID:     "ws-1",
			ChannelStrategy: participation.StrategyNamed,
			ChannelID:       "builders",
			Source:          participation.SourceExplicitRequest,
			Bounds: participation.Bounds{
				MaxWakes:         4,
				MaxWakeWallTime:  "30s",
				MaxTotalWallTime: "2m",
				MaxInputTokens:   4096,
				MaxOutputTokens:  4096,
				MaxWakeDepth:     4,
				CoalesceWindow:   "250ms",
			},
		}
		info := &session.Info{
			ID:                   "sess-1",
			ProfileID:            "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			Name:                 "worker",
			AgentName:            "coder",
			Provider:             "provider",
			Model:                "gpt-5.4",
			WorkspaceID:          "ws-1",
			Workspace:            "/workspace",
			NetworkParticipation: wantParticipation,
			Type:                 session.SessionTypeUser,
			State:                session.StateActive,
			SoulSnapshotID:       "soul-1",
			SoulDigest:           "digest-1",
			ParentSoulDigest:     "digest-parent",
			CreatedAt:            now,
			UpdatedAt:            now,
		}

		got := SessionSnapshotFromInfo(info)
		info.NetworkParticipation.ChannelID = "mutated-after-conversion"
		if got.ID != info.ID ||
			got.ProfileID != info.ProfileID ||
			got.Name != info.Name ||
			got.AgentName != info.AgentName ||
			got.Provider != info.Provider ||
			got.Model != info.Model ||
			got.WorkspaceID != info.WorkspaceID ||
			got.WorkspacePath != info.Workspace ||
			got.NetworkSpecSnapshot() != wantParticipation ||
			got.Type != info.Type ||
			got.State != info.State ||
			got.SoulSnapshotID != info.SoulSnapshotID ||
			got.SoulDigest != info.SoulDigest ||
			got.ParentSoulDigest != info.ParentSoulDigest ||
			!got.CreatedAt.Equal(now) ||
			!got.UpdatedAt.Equal(now) {
			t.Fatalf("SessionSnapshotFromInfo() = %#v, want fields copied from session.Info", got)
		}
	})
}

func TestErrorPayloadFallbacksAndExitCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantCode   string
		wantMsg    string
		wantAction string
		wantExit   int
	}{
		{
			name:       "Should return ok defaults for nil error",
			wantCode:   "agent_error",
			wantMsg:    agentCommandFailedMessage,
			wantAction: "inspect the daemon error and retry",
			wantExit:   ExitOK,
		},
		{
			name:       "Should return generic unavailable payload for ordinary errors",
			err:        errors.New("daemon unavailable"),
			wantCode:   "agent_error",
			wantMsg:    agentCommandFailedMessage,
			wantAction: "inspect the daemon error and retry",
			wantExit:   ExitUnavailable,
		},
		{
			name:       "Should preserve identity exit code with fallback text",
			err:        &Error{Err: ErrIdentityRequired},
			wantCode:   "agent_error",
			wantMsg:    agentCommandFailedMessage,
			wantAction: "inspect the daemon error and retry",
			wantExit:   ExitIdentityRequired,
		},
		{
			name:       "Should map diagnostic identity code without sentinel to identity exit",
			err:        &Error{Code: contract.CodeIdentityRequired},
			wantCode:   contract.CodeIdentityRequired,
			wantMsg:    agentCommandFailedMessage,
			wantAction: "inspect the daemon error and retry",
			wantExit:   ExitIdentityRequired,
		},
		{
			name: "Should map lookup unavailable identity errors to unavailable exit",
			err: identityError(
				ErrIdentityLookupUnavailable,
				"identity_lookup_unavailable",
				"agent identity cannot be validated",
				"retry after the daemon is reachable",
			),
			wantCode:   "identity_lookup_unavailable",
			wantMsg:    "agent identity cannot be validated",
			wantAction: "retry after the daemon is reachable",
			wantExit:   ExitUnavailable,
		},
		{
			name: "Should map config invalid diagnostic to config invalid exit",
			err: diagnostics.NewStructuredError(diagnostics.NewItem(diagnostics.ItemSpec{
				ID:            "config.parse",
				Code:          contract.CodeConfigInvalid,
				Category:      contract.CategoryConfig,
				Title:         "Config invalid",
				Message:       "config token=secret failed",
				Severity:      contract.SeverityCritical,
				DataFreshness: contract.FreshnessLive,
			}), errors.New("parse failed")),
			wantCode:   "agent_error",
			wantMsg:    agentCommandFailedMessage,
			wantAction: "inspect the daemon error and retry",
			wantExit:   ExitConfigInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ExitCodeForError(tt.err); got != tt.wantExit {
				t.Fatalf("ExitCodeForError() = %d, want %d", got, tt.wantExit)
			}
			payload := ErrorPayloadFor(tt.err)
			if payload.Code != tt.wantCode ||
				payload.Message != tt.wantMsg ||
				payload.Action != tt.wantAction ||
				payload.ExitCode != tt.wantExit {
				t.Fatalf("ErrorPayloadFor() = %#v, want code=%q message=%q action=%q exit=%d",
					payload,
					tt.wantCode,
					tt.wantMsg,
					tt.wantAction,
					tt.wantExit,
				)
			}
		})
	}
}
