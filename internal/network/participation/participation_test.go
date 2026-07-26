package participation_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

func TestValidateShouldEnforceParticipationContract(t *testing.T) {
	t.Parallel()

	validBounds := completeBoundsRequest()
	tests := []struct {
		name    string
		request participation.Request
		wantErr error
		check   func(*testing.T, participation.Request)
	}{
		{
			name: "Should normalize a valid named live request",
			request: participation.Request{
				Mode:            new(participation.ModeLive),
				ChannelStrategy: new(participation.StrategyNamed),
				ChannelID:       new(" builders "),
				Bounds:          validBounds,
			},
			check: func(t *testing.T, got participation.Request) {
				t.Helper()
				if got.ChannelID == nil || *got.ChannelID != "builders" {
					t.Fatalf("Validate() ChannelID = %v, want builders", got.ChannelID)
				}
			},
		},
		{
			name: "Should reject local mode with a channel",
			request: participation.Request{
				Mode:      new(participation.ModeLocal),
				ChannelID: new("builders"),
			},
			wantErr: participation.ErrStrategyChannelConflict,
		},
		{
			name: "Should reject live mode without a strategy",
			request: participation.Request{
				Mode:   new(participation.ModeLive),
				Bounds: validBounds,
			},
			wantErr: participation.ErrStrategyInvalid,
		},
		{
			name: "Should reject named strategy without a channel",
			request: participation.Request{
				Mode:            new(participation.ModeLive),
				ChannelStrategy: new(participation.StrategyNamed),
				Bounds:          validBounds,
			},
			wantErr: participation.ErrStrategyInvalid,
		},
		{
			name: "Should reject a derived strategy with a channel",
			request: participation.Request{
				Mode:            new(participation.ModeLive),
				ChannelStrategy: new(participation.StrategyRun),
				ChannelID:       new("builders"),
				Bounds:          validBounds,
			},
			wantErr: participation.ErrStrategyChannelConflict,
		},
		{
			name: "Should reject live mode without finite bounds",
			request: participation.Request{
				Mode:            new(participation.ModeLive),
				ChannelStrategy: new(participation.StrategyRun),
			},
			wantErr: participation.ErrBoundsExceedCeiling,
		},
		{
			name: "Should reject an unknown mode and list allowed values",
			request: participation.Request{
				Mode: new(participation.Mode("mailbox")),
			},
			wantErr: participation.ErrStrategyInvalid,
			check: func(t *testing.T, _ participation.Request) {
				t.Helper()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := participation.Validate(tc.request)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Validate() error = %v, want errors.Is(%v)", err, tc.wantErr)
				}
				if tc.request.Mode != nil && *tc.request.Mode == participation.Mode("mailbox") &&
					(!strings.Contains(err.Error(), "local") || !strings.Contains(err.Error(), "live")) {
					t.Fatalf("Validate() error = %q, want allowed modes", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}

func TestCloneRequestShouldDeepCopyIntent(t *testing.T) {
	t.Parallel()

	request := participation.Request{
		Mode:            new(participation.ModeLive),
		ChannelStrategy: new(participation.StrategyNamed),
		ChannelID:       new("builders"),
		Bounds:          completeBoundsRequest(),
	}
	cloned := participation.CloneRequest(&request)
	if cloned == nil || cloned.Bounds == nil {
		t.Fatalf("CloneRequest() = %#v, want complete copy", cloned)
	}
	*cloned.ChannelID = "changed"
	*cloned.Bounds.MaxWakes = 99
	if got := *request.ChannelID; got != "builders" {
		t.Fatalf("source ChannelID = %q after clone mutation, want builders", got)
	}
	if got := *request.Bounds.MaxWakes; got == 99 {
		t.Fatalf("source MaxWakes = %d after clone mutation, want independent value", got)
	}
}

func TestParticipationPublicHelpersShouldPreserveCanonicalSnapshots(t *testing.T) {
	t.Parallel()

	local := participation.LocalSpec()
	if err := participation.ValidateSpec(local); err != nil {
		t.Fatalf("ValidateSpec(LocalSpec()) error = %v", err)
	}
	if local.Version != participation.SpecVersion || local.Mode != participation.ModeLocal ||
		local.Source != participation.SourceBuiltInLocal {
		t.Fatalf("LocalSpec() = %#v, want canonical built-in Local", local)
	}
	cloned := participation.CloneSpec(local)
	if cloned == nil || *cloned != local {
		t.Fatalf("CloneSpec() = %#v, want %#v", cloned, local)
	}
	cloned.Source = participation.SourceExplicitRequest
	if local.Source != participation.SourceBuiltInLocal {
		t.Fatalf("CloneSpec() aliased source snapshot: %#v", local)
	}

	for _, test := range []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "Should accept a canonical session owner", key: "session:sess-1"},
		{name: "Should accept a canonical task-run owner", key: "task_run:run-1"},
		{name: "Should reject a missing separator", key: "run-1", wantErr: true},
		{name: "Should reject an unsupported kind", key: "other:run-1", wantErr: true},
		{name: "Should reject an empty id", key: "session:", wantErr: true},
		{name: "Should reject noncanonical id whitespace", key: "session: sess-1", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := participation.ValidateOwnerKey(test.key)
			if test.wantErr && err == nil {
				t.Fatalf("ValidateOwnerKey(%q) error = nil, want non-nil", test.key)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("ValidateOwnerKey(%q) error = %v", test.key, err)
			}
		})
	}
}

func TestNormalizeIntentShouldValidatePartialPersistedIntent(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve an omitted intent", func(t *testing.T) {
		t.Parallel()
		got, err := participation.NormalizeIntent(participation.Request{})
		if err != nil || got.Mode != nil || got.ChannelStrategy != nil || got.ChannelID != nil || got.Bounds != nil {
			t.Fatalf("NormalizeIntent(empty) = %#v, error=%v", got, err)
		}
	})

	t.Run("Should normalize a partial named Live intent without filling defaults", func(t *testing.T) {
		t.Parallel()
		maxWakes := 2
		wakeWall := " 30s "
		mode := participation.Mode(" live ")
		strategy := participation.ChannelStrategy(" named ")
		channel := " builders "
		got, err := participation.NormalizeIntent(participation.Request{
			Mode: &mode, ChannelStrategy: &strategy, ChannelID: &channel,
			Bounds: &participation.BoundsRequest{MaxWakes: &maxWakes, MaxWakeWallTime: &wakeWall},
		})
		if err != nil {
			t.Fatalf("NormalizeIntent(partial Live) error = %v", err)
		}
		if got.Mode == nil || *got.Mode != participation.ModeLive ||
			got.ChannelStrategy == nil || *got.ChannelStrategy != participation.StrategyNamed ||
			got.ChannelID == nil || *got.ChannelID != "builders" || got.Bounds == nil ||
			got.Bounds.MaxWakeWallTime == nil || *got.Bounds.MaxWakeWallTime != "30s" ||
			got.Bounds.MaxInputTokens != nil {
			t.Fatalf("NormalizeIntent(partial Live) = %#v, want normalized partial values only", got)
		}
	})

	validPartial := func() participation.Request {
		return participation.Request{
			Mode:            new(participation.ModeLive),
			ChannelStrategy: new(participation.StrategyRun),
		}
	}
	for _, test := range []struct {
		name    string
		request func() participation.Request
		wantErr error
	}{
		{
			name: "Should reject Local channel state",
			request: func() participation.Request {
				return participation.Request{Mode: new(participation.ModeLocal), ChannelID: new("builders")}
			},
			wantErr: participation.ErrStrategyChannelConflict,
		},
		{
			name: "Should reject an unknown mode",
			request: func() participation.Request {
				return participation.Request{Mode: new(participation.Mode("remote"))}
			},
			wantErr: participation.ErrStrategyInvalid,
		},
		{
			name: "Should require a Live strategy",
			request: func() participation.Request {
				return participation.Request{Mode: new(participation.ModeLive)}
			},
			wantErr: participation.ErrStrategyInvalid,
		},
		{
			name: "Should reject an unknown strategy",
			request: func() participation.Request {
				return participation.Request{
					Mode: new(participation.ModeLive), ChannelStrategy: new(participation.ChannelStrategy("other")),
				}
			},
			wantErr: participation.ErrStrategyInvalid,
		},
		{
			name: "Should require a named channel",
			request: func() participation.Request {
				return participation.Request{
					Mode: new(participation.ModeLive), ChannelStrategy: new(participation.StrategyNamed),
				}
			},
			wantErr: participation.ErrStrategyInvalid,
		},
		{
			name: "Should reject an invalid named channel",
			request: func() participation.Request {
				return participation.Request{
					Mode: new(participation.ModeLive), ChannelStrategy: new(participation.StrategyNamed),
					ChannelID: new("Invalid Channel"),
				}
			},
			wantErr: participation.ErrStrategyInvalid,
		},
		{
			name: "Should reject a channel on a derived strategy",
			request: func() participation.Request {
				request := validPartial()
				request.ChannelID = new("builders")
				return request
			},
			wantErr: participation.ErrStrategyChannelConflict,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := participation.NormalizeIntent(test.request()); !errors.Is(err, test.wantErr) {
				t.Fatalf("NormalizeIntent() error = %v, want errors.Is(%v)", err, test.wantErr)
			}
		})
	}

	for _, test := range []struct {
		name   string
		bounds participation.BoundsRequest
	}{
		{name: "Should reject nonpositive wakes", bounds: participation.BoundsRequest{MaxWakes: new(0)}},
		{name: "Should reject nonpositive input tokens", bounds: participation.BoundsRequest{MaxInputTokens: new(int64(0))}},
		{name: "Should reject nonpositive output tokens", bounds: participation.BoundsRequest{MaxOutputTokens: new(int64(-1))}},
		{name: "Should reject nonpositive wake depth", bounds: participation.BoundsRequest{MaxWakeDepth: new(0)}},
		{name: "Should reject invalid wake wall time", bounds: participation.BoundsRequest{MaxWakeWallTime: new("bad")}},
		{name: "Should reject invalid total wall time", bounds: participation.BoundsRequest{MaxTotalWallTime: new("0s")}},
		{name: "Should reject invalid coalesce window", bounds: participation.BoundsRequest{CoalesceWindow: new("-1s")}},
		{
			name: "Should reject input tokens above the JavaScript safe integer ceiling",
			bounds: participation.BoundsRequest{
				MaxInputTokens: new(int64(9_007_199_254_740_992)),
			},
		},
		{
			name: "Should reject output tokens above the JavaScript safe integer ceiling",
			bounds: participation.BoundsRequest{
				MaxOutputTokens: new(int64(9_007_199_254_740_992)),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := validPartial()
			request.Bounds = &test.bounds
			if _, err := participation.NormalizeIntent(request); !errors.Is(err, participation.ErrBoundsExceedCeiling) {
				t.Fatalf("NormalizeIntent() error = %v, want ErrBoundsExceedCeiling", err)
			}
		})
	}
}

func TestObserveResolvedShouldDispatchOnlyToObservers(t *testing.T) {
	t.Parallel()

	observation := participation.ResolvedObservation{
		WorkspaceID: "ws-1",
		Owner: participation.OwnerRef{
			WorkspaceID: "ws-1", Kind: participation.OwnerKindSession, ID: "sess-1",
		},
		Spec: participation.LocalSpec(),
	}
	plain := participation.Resolver(participationResolverStub{})
	if err := participation.ObserveResolved(t.Context(), plain, observation); err != nil {
		t.Fatalf("ObserveResolved(non-observer) error = %v", err)
	}
	wantErr := errors.New("observer failed")
	observer := &participationObserverStub{err: wantErr}
	if err := participation.ObserveResolved(t.Context(), observer, observation); !errors.Is(err, wantErr) {
		t.Fatalf("ObserveResolved(observer) error = %v, want %v", err, wantErr)
	}
	if observer.observation != observation {
		t.Fatalf("observed snapshot = %#v, want %#v", observer.observation, observation)
	}
}

func TestValidateShouldRejectInvalidDurations(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"0s", "-1s", "250"} {
		t.Run("Should reject "+value, func(t *testing.T) {
			t.Parallel()

			bounds := completeBoundsRequest()
			bounds.CoalesceWindow = &value
			_, err := participation.Validate(participation.Request{
				Mode:            new(participation.ModeLive),
				ChannelStrategy: new(participation.StrategyRun),
				Bounds:          bounds,
			})
			if !errors.Is(err, participation.ErrBoundsExceedCeiling) {
				t.Fatalf("Validate() error = %v, want ErrBoundsExceedCeiling", err)
			}
			if !strings.Contains(err.Error(), "coalesce_window") {
				t.Fatalf("Validate() error = %q, want field name", err)
			}
		})
	}

	t.Run("Should accept a positive subsecond duration", func(t *testing.T) {
		t.Parallel()

		bounds := completeBoundsRequest()
		bounds.CoalesceWindow = new("250ms")
		if _, err := participation.Validate(participation.Request{
			Mode:            new(participation.ModeLive),
			ChannelStrategy: new(participation.StrategyRun),
			Bounds:          bounds,
		}); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	})

	t.Run("Should reject a duration that can overflow millisecond ceiling conversion", func(t *testing.T) {
		t.Parallel()

		value := time.Duration(1<<63 - 1).String()
		bounds := completeBoundsRequest()
		bounds.MaxWakeWallTime = &value
		_, err := participation.Validate(participation.Request{
			Mode:            new(participation.ModeLive),
			ChannelStrategy: new(participation.StrategyRun),
			Bounds:          bounds,
		})
		if !errors.Is(err, participation.ErrBoundsExceedCeiling) {
			t.Fatalf("Validate() error = %v, want ErrBoundsExceedCeiling", err)
		}
		if !strings.Contains(err.Error(), "max_wake_wall_time") {
			t.Fatalf("Validate() error = %q, want max_wake_wall_time", err)
		}
	})
}

func TestValidateShouldKeepWakeWallWithinTotalWall(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		wakeWall  string
		totalWall string
		wantErr   bool
	}{
		{name: "Should accept wake wall below total wall", wakeWall: "4m", totalWall: "5m"},
		{name: "Should accept wake wall equal to total wall", wakeWall: "5m", totalWall: "5m"},
		{name: "Should reject wake wall above total wall", wakeWall: "6m", totalWall: "5m", wantErr: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			bounds := completeBoundsRequest()
			bounds.MaxWakeWallTime = &testCase.wakeWall
			bounds.MaxTotalWallTime = &testCase.totalWall
			_, err := participation.Validate(participation.Request{
				Mode:            new(participation.ModeLive),
				ChannelStrategy: new(participation.StrategyRun),
				Bounds:          bounds,
			})
			if !testCase.wantErr {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if !errors.Is(err, participation.ErrBoundsExceedCeiling) {
				t.Fatalf("Validate() error = %v, want ErrBoundsExceedCeiling", err)
			}
			if !strings.Contains(err.Error(), "max_wake_wall_time") ||
				!strings.Contains(err.Error(), "max_total_wall_time") {
				t.Fatalf("Validate() error = %q, want both wall-time fields", err)
			}
		})
	}

	t.Run("Should reject contradictory partial authored bounds", func(t *testing.T) {
		t.Parallel()

		wakeWall := "6m"
		totalWall := "5m"
		_, err := participation.NormalizeIntent(participation.Request{
			Mode:            new(participation.ModeLive),
			ChannelStrategy: new(participation.StrategyRun),
			Bounds: &participation.BoundsRequest{
				MaxWakeWallTime:  &wakeWall,
				MaxTotalWallTime: &totalWall,
			},
		})
		if !errors.Is(err, participation.ErrBoundsExceedCeiling) {
			t.Fatalf("NormalizeIntent() error = %v, want ErrBoundsExceedCeiling", err)
		}
		if !strings.Contains(err.Error(), "max_wake_wall_time") ||
			!strings.Contains(err.Error(), "max_total_wall_time") {
			t.Fatalf("NormalizeIntent() error = %q, want both wall-time fields", err)
		}
	})
}

func TestResolverShouldResolveLocalWithoutChannelLookup(t *testing.T) {
	t.Parallel()

	lookups := 0
	resolver := newTestResolver(t, participation.ResolverOptions{
		ChannelExists: func(context.Context, string, string) (bool, error) {
			lookups++
			return true, nil
		},
	})
	spec, err := resolver.Resolve(context.Background(), participation.ResolveInput{
		WorkspaceID: "ws-1",
		Owner: participation.OwnerRef{
			WorkspaceID: "ws-1",
			Kind:        participation.OwnerKindSession,
			ID:          "session-1",
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if spec.Mode != participation.ModeLocal || spec.Source != participation.SourceBuiltInLocal {
		t.Fatalf("Resolve() spec = %#v, want built-in local", spec)
	}
	if spec.ChannelID != "" || !spec.Bounds.IsZero() {
		t.Fatalf("Resolve() spec = %#v, want no channel or bounds", spec)
	}
	if lookups != 0 {
		t.Fatalf("Resolve() channel lookups = %d, want 0", lookups)
	}
}

func TestOwnerRefShouldRequireWorkspaceQualifiedIdentity(t *testing.T) {
	t.Parallel()

	valid := participation.OwnerRef{
		WorkspaceID: "ws-a",
		Kind:        participation.OwnerKindTaskRun,
		ID:          "run-1",
	}
	if err := participation.ValidateOwner(valid); err != nil {
		t.Fatalf("ValidateOwner(valid) error = %v", err)
	}
	if got := participation.OwnerKey(valid); got != "task_run:run-1" {
		t.Fatalf("OwnerKey(valid) = %q, want task_run:run-1", got)
	}
	projected, err := participation.OwnerRefFromKey(valid.WorkspaceID, participation.OwnerKey(valid))
	if err != nil {
		t.Fatalf("OwnerRefFromKey(valid) error = %v", err)
	}
	if projected != valid {
		t.Fatalf("OwnerRefFromKey(valid) = %#v, want %#v", projected, valid)
	}

	for _, testCase := range []struct {
		name  string
		owner participation.OwnerRef
		field string
	}{
		{name: "missing workspace", owner: participation.OwnerRef{Kind: valid.Kind, ID: valid.ID}, field: "workspace_id"},
		{name: "blank workspace", owner: participation.OwnerRef{WorkspaceID: "  ", Kind: valid.Kind, ID: valid.ID}, field: "workspace_id"},
		{name: "unknown kind", owner: participation.OwnerRef{WorkspaceID: valid.WorkspaceID, Kind: "other", ID: valid.ID}, field: "kind"},
		{name: "missing id", owner: participation.OwnerRef{WorkspaceID: valid.WorkspaceID, Kind: valid.Kind}, field: "id"},
		{name: "blank id", owner: participation.OwnerRef{WorkspaceID: valid.WorkspaceID, Kind: valid.Kind, ID: "  "}, field: "id"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := participation.ValidateOwner(testCase.owner)
			if err == nil || !strings.Contains(err.Error(), testCase.field) {
				t.Fatalf("ValidateOwner(%#v) error = %v, want field %q", testCase.owner, err, testCase.field)
			}
		})
	}

	t.Run("Should reject an owner key without a workspace", func(t *testing.T) {
		t.Parallel()
		if _, err := participation.OwnerRefFromKey("", "task_run:run-1"); err == nil ||
			!strings.Contains(err.Error(), "workspace_id") {
			t.Fatalf("OwnerRefFromKey(empty workspace) error = %v, want workspace_id", err)
		}
	})
}

func TestResolverShouldRejectOwnerFromAnotherWorkspace(t *testing.T) {
	t.Parallel()

	resolver := newTestResolver(t, participation.ResolverOptions{})
	_, err := resolver.Resolve(context.Background(), participation.ResolveInput{
		WorkspaceID: "ws-a",
		Owner: participation.OwnerRef{
			WorkspaceID: "ws-b",
			Kind:        participation.OwnerKindSession,
			ID:          "session-1",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("Resolve(cross-workspace owner) error = %v, want workspace mismatch", err)
	}
}

func TestResolverShouldRejectRunStrategyWithoutRunID(t *testing.T) {
	t.Parallel()

	resolver := newTestResolver(t, participation.ResolverOptions{})
	_, err := resolver.Resolve(context.Background(), participation.ResolveInput{
		WorkspaceID: "ws-1",
		Owner: participation.OwnerRef{
			WorkspaceID: "ws-1",
			Kind:        participation.OwnerKindTaskRun,
			ID:          "owner-1",
		},
		Request: &participation.Request{
			Mode:            new(participation.ModeLive),
			ChannelStrategy: new(participation.StrategyRun),
		},
	})
	if !errors.Is(err, participation.ErrStrategyInvalid) {
		t.Fatalf("Resolve() error = %v, want ErrStrategyInvalid", err)
	}
	if !strings.Contains(err.Error(), "task_run") {
		t.Fatalf("Resolve() error = %q, want owner kind", err)
	}
}

func TestResolverShouldDeriveUniqueChannelIDs(t *testing.T) {
	t.Parallel()

	resolver := newTestResolver(t, participation.ResolverOptions{})
	resolve := func(runID string) participation.Spec {
		t.Helper()
		spec, err := resolver.Resolve(context.Background(), participation.ResolveInput{
			WorkspaceID: "ws-1",
			Owner: participation.OwnerRef{
				WorkspaceID: "ws-1",
				Kind:        participation.OwnerKindTaskRun,
				ID:          runID,
			},
			RunID: runID,
			Request: &participation.Request{
				Mode:            new(participation.ModeLive),
				ChannelStrategy: new(participation.StrategyRun),
			},
		})
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", runID, err)
		}
		if len(spec.ChannelID) > 64 {
			t.Fatalf("Resolve(%q) ChannelID length = %d, want <= 64", runID, len(spec.ChannelID))
		}
		return spec
	}

	caseVariant := resolve("Run-Release")
	caseVariantLower := resolve("run-release")
	if caseVariant.ChannelID == caseVariantLower.ChannelID {
		t.Fatalf("case-variant run IDs derived the same channel %q", caseVariant.ChannelID)
	}

	sharedPrefix := strings.Repeat("release-segment-", 5)
	longA := resolve(sharedPrefix + "alpha")
	longB := resolve(sharedPrefix + "bravo")
	if longA.ChannelID == longB.ChannelID {
		t.Fatalf("long run IDs derived the same channel %q", longA.ChannelID)
	}
	if repeated := resolve("Run-Release"); repeated.ChannelID != caseVariant.ChannelID {
		t.Fatalf("same run ID derived %q then %q", caseVariant.ChannelID, repeated.ChannelID)
	}
}

func TestResolverShouldEnforceOwnerPrecedenceAndCapabilityGates(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve explicit Local provenance", func(t *testing.T) {
		t.Parallel()

		resolver := newTestResolver(t, participation.ResolverOptions{})
		spec, err := resolver.Resolve(context.Background(), participation.ResolveInput{
			WorkspaceID: "ws-1",
			Owner: participation.OwnerRef{
				WorkspaceID: "ws-1",
				Kind:        participation.OwnerKindSession,
				ID:          "session-1",
			},
			Request: &participation.Request{Mode: new(participation.ModeLocal)},
		})
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if spec.Mode != participation.ModeLocal || spec.Source != participation.SourceExplicitRequest {
			t.Fatalf("Resolve() = %#v, want explicit Local", spec)
		}
	})

	t.Run("Should preserve automation job provenance for definition-owned requests", func(t *testing.T) {
		t.Parallel()

		resolver := newTestResolver(t, participation.ResolverOptions{})
		spec, err := resolver.Resolve(context.Background(), participation.ResolveInput{
			WorkspaceID: "ws-1",
			Owner: participation.OwnerRef{
				WorkspaceID: "ws-1",
				Kind:        participation.OwnerKindAutomationRun,
				ID:          "automation-run-1",
			},
			Request:       &participation.Request{Mode: new(participation.ModeLocal)},
			RequestSource: participation.SourceAutomationJob,
		})
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if spec.Mode != participation.ModeLocal || spec.Source != participation.SourceAutomationJob {
			t.Fatalf("Resolve() = %#v, want automation-job Local", spec)
		}
	})

	t.Run("Should reject non-owner request provenance", func(t *testing.T) {
		t.Parallel()

		resolver := newTestResolver(t, participation.ResolverOptions{})
		_, err := resolver.Resolve(context.Background(), participation.ResolveInput{
			WorkspaceID: "ws-1",
			Owner: participation.OwnerRef{
				WorkspaceID: "ws-1",
				Kind:        participation.OwnerKindTaskRun,
				ID:          "run-1",
			},
			Request:       &participation.Request{Mode: new(participation.ModeLocal)},
			RequestSource: participation.SourceTaskProfile,
		})
		if err == nil || !strings.Contains(err.Error(), "request source") {
			t.Fatalf("Resolve() error = %v, want rejected request source", err)
		}
	})

	t.Run("Should not consult workspace coordination for non-coordinated task runs", func(t *testing.T) {
		t.Parallel()

		coordinationReads := 0
		resolver := newTestResolver(t, participation.ResolverOptions{
			WorkspaceCoordination: func(context.Context, string) (bool, error) {
				coordinationReads++
				return true, nil
			},
		})
		spec, err := resolver.Resolve(context.Background(), participation.ResolveInput{
			WorkspaceID: "ws-1",
			Owner: participation.OwnerRef{
				WorkspaceID: "ws-1",
				Kind:        participation.OwnerKindTaskRun,
				ID:          "run-1",
			},
			RunID: "run-1",
		})
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if spec.Mode != participation.ModeLocal || spec.Source != participation.SourceBuiltInLocal {
			t.Fatalf("Resolve() = %#v, want built-in Local", spec)
		}
		if coordinationReads != 0 {
			t.Fatalf("workspace coordination reads = %d, want 0", coordinationReads)
		}
	})

	t.Run("Should reject unavailable Live before channel or authority lookups", func(t *testing.T) {
		t.Parallel()

		channelReads := 0
		authorityReads := 0
		resolver := newTestResolver(t, participation.ResolverOptions{
			Availability: func(context.Context) (bool, error) { return false, nil },
			ChannelExists: func(context.Context, string, string) (bool, error) {
				channelReads++
				return true, nil
			},
			Authority: func(context.Context, participation.ResolveInput, participation.Spec) (bool, error) {
				authorityReads++
				return true, nil
			},
		})
		_, err := resolver.Resolve(context.Background(), participation.ResolveInput{
			WorkspaceID: "ws-1",
			Owner: participation.OwnerRef{
				WorkspaceID: "ws-1",
				Kind:        participation.OwnerKindSession,
				ID:          "session-1",
			},
			Request: &participation.Request{
				Mode:            new(participation.ModeLive),
				ChannelStrategy: new(participation.StrategyNamed),
				ChannelID:       new("builders"),
			},
		})
		if !errors.Is(err, participation.ErrUnavailable) {
			t.Fatalf("Resolve() error = %v, want ErrUnavailable", err)
		}
		if channelReads != 0 || authorityReads != 0 {
			t.Fatalf("post-availability reads = channel:%d authority:%d, want zero", channelReads, authorityReads)
		}
	})

	t.Run("Should derive one loop-run conversation from loop definition intent", func(t *testing.T) {
		t.Parallel()

		resolver := newTestResolver(t, participation.ResolverOptions{})
		spec, err := resolver.Resolve(context.Background(), participation.ResolveInput{
			WorkspaceID: "ws-1",
			Owner: participation.OwnerRef{
				WorkspaceID: "ws-1",
				Kind:        participation.OwnerKindLoopRun,
				ID:          "loop-run-1",
			},
			LoopRunID: "loop-run-1",
			Definition: &participation.Request{
				Mode:            new(participation.ModeLive),
				ChannelStrategy: new(participation.StrategyLoopRun),
			},
		})
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if spec.Mode != participation.ModeLive || spec.Source != participation.SourceLoopDefinition ||
			spec.ChannelID == "" {
			t.Fatalf("Resolve() = %#v, want Live loop-definition conversation", spec)
		}
	})

	t.Run("Should reject an unknown named channel without creating it", func(t *testing.T) {
		t.Parallel()

		lookups := 0
		resolver := newTestResolver(t, participation.ResolverOptions{
			ChannelExists: func(context.Context, string, string) (bool, error) {
				lookups++
				return false, nil
			},
		})
		_, err := resolver.Resolve(context.Background(), participation.ResolveInput{
			WorkspaceID: "ws-1",
			Owner: participation.OwnerRef{
				WorkspaceID: "ws-1",
				Kind:        participation.OwnerKindSession,
				ID:          "session-1",
			},
			Request: &participation.Request{
				Mode:            new(participation.ModeLive),
				ChannelStrategy: new(participation.StrategyNamed),
				ChannelID:       new("missing"),
			},
		})
		if !errors.Is(err, participation.ErrChannelUnknown) {
			t.Fatalf("Resolve() error = %v, want ErrChannelUnknown", err)
		}
		if lookups != 1 {
			t.Fatalf("channel lookups = %d, want 1", lookups)
		}
	})

	t.Run("Should apply request over profile and profile over workspace", func(t *testing.T) {
		t.Parallel()

		resolver := newTestResolver(t, participation.ResolverOptions{
			WorkspaceCoordination: func(context.Context, string) (bool, error) { return true, nil },
		})
		profileLive := &participation.Request{
			Mode:            new(participation.ModeLive),
			ChannelStrategy: new(participation.StrategyRun),
		}
		explicitLocal, err := resolver.Resolve(context.Background(), participation.ResolveInput{
			WorkspaceID: "ws-1",
			Owner: participation.OwnerRef{
				WorkspaceID: "ws-1",
				Kind:        participation.OwnerKindTaskRun,
				ID:          "run-1",
			},
			RunID:       "run-1",
			Coordinated: true,
			Request:     &participation.Request{Mode: new(participation.ModeLocal)},
			Definition:  profileLive,
		})
		if err != nil {
			t.Fatalf("Resolve(explicit Local) error = %v", err)
		}
		if explicitLocal.Source != participation.SourceExplicitRequest ||
			explicitLocal.Mode != participation.ModeLocal {
			t.Fatalf("Resolve(explicit Local) = %#v", explicitLocal)
		}
		profileLocal, err := resolver.Resolve(context.Background(), participation.ResolveInput{
			WorkspaceID: "ws-1",
			Owner: participation.OwnerRef{
				WorkspaceID: "ws-1",
				Kind:        participation.OwnerKindTaskRun,
				ID:          "run-2",
			},
			RunID:       "run-2",
			Coordinated: true,
			Definition:  &participation.Request{Mode: new(participation.ModeLocal)},
		})
		if err != nil {
			t.Fatalf("Resolve(profile Local) error = %v", err)
		}
		if profileLocal.Source != participation.SourceTaskProfile || profileLocal.Mode != participation.ModeLocal {
			t.Fatalf("Resolve(profile Local) = %#v", profileLocal)
		}
	})

	t.Run("Should reject Live outside delegated authority", func(t *testing.T) {
		t.Parallel()

		resolver := newTestResolver(t, participation.ResolverOptions{
			ChannelExists: func(context.Context, string, string) (bool, error) { return true, nil },
			Authority: func(context.Context, participation.ResolveInput, participation.Spec) (bool, error) {
				return false, nil
			},
		})
		_, err := resolver.Resolve(context.Background(), participation.ResolveInput{
			WorkspaceID: "ws-1",
			Owner: participation.OwnerRef{
				WorkspaceID: "ws-1",
				Kind:        participation.OwnerKindSession,
				ID:          "child-1",
			},
			Request: &participation.Request{
				Mode:            new(participation.ModeLive),
				ChannelStrategy: new(participation.StrategyNamed),
				ChannelID:       new("private"),
			},
		})
		if !errors.Is(err, participation.ErrAuthorityDenied) {
			t.Fatalf("Resolve() error = %v, want ErrAuthorityDenied", err)
		}
	})

	t.Run("Should keep independent children Local without parent input", func(t *testing.T) {
		t.Parallel()

		resolver := newTestResolver(t, participation.ResolverOptions{})
		for _, childID := range []string{"spawn-1", "review-1", "detached-1"} {
			spec, err := resolver.Resolve(context.Background(), participation.ResolveInput{
				WorkspaceID: "ws-1",
				Owner: participation.OwnerRef{
					WorkspaceID: "ws-1",
					Kind:        participation.OwnerKindSession,
					ID:          childID,
				},
			})
			if err != nil {
				t.Fatalf("Resolve(%s) error = %v", childID, err)
			}
			if spec.Mode != participation.ModeLocal || spec.Source != participation.SourceBuiltInLocal {
				t.Fatalf("Resolve(%s) = %#v, want independent Local", childID, spec)
			}
		}
	})

	t.Run("Should reject owners that cannot honor bounded Live", func(t *testing.T) {
		t.Parallel()

		resolver := newTestResolver(t, participation.ResolverOptions{
			LiveSupport: func(context.Context, participation.ResolveInput) (bool, error) { return false, nil },
		})
		_, err := resolver.Resolve(context.Background(), participation.ResolveInput{
			WorkspaceID: "ws-1",
			Owner: participation.OwnerRef{
				WorkspaceID: "ws-1",
				Kind:        participation.OwnerKindTaskRun,
				ID:          "run-unsupported",
			},
			RunID: "run-unsupported",
			Request: &participation.Request{
				Mode:            new(participation.ModeLive),
				ChannelStrategy: new(participation.StrategyRun),
			},
		})
		if !errors.Is(err, participation.ErrLiveUnsupported) || !strings.Contains(err.Error(), "local") {
			t.Fatalf("Resolve() error = %v, want ErrLiveUnsupported naming Local", err)
		}
	})
}

func TestSpecShouldRoundTripLosslessly(t *testing.T) {
	t.Parallel()

	want := participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     "ws-1",
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       "builders",
		Source:          participation.SourceExplicitRequest,
		Bounds:          completeBounds(),
	}
	payload, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(payload), `"version":"network-participation/v1"`) {
		t.Fatalf("json.Marshal() = %s, want canonical version", payload)
	}
	var got participation.Spec
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got != want {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
	if err := participation.ValidateSpec(got); err != nil {
		t.Fatalf("ValidateSpec() error = %v", err)
	}
}

func TestValidateSpecShouldRejectProjectionConflicts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    participation.Spec
		wantErr error
	}{
		{
			name: "Should reject a local snapshot carrying a channel",
			spec: participation.Spec{
				Version:   participation.SpecVersion,
				Mode:      participation.ModeLocal,
				ChannelID: "builders",
				Source:    participation.SourceBuiltInLocal,
			},
			wantErr: participation.ErrStrategyChannelConflict,
		},
		{
			name: "Should reject a snapshot with an unknown version",
			spec: participation.Spec{
				Version: "network-participation/v2",
				Mode:    participation.ModeLocal,
				Source:  participation.SourceBuiltInLocal,
			},
			wantErr: participation.ErrStrategyInvalid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := participation.ValidateSpec(tc.spec); !errors.Is(err, tc.wantErr) {
				t.Fatalf("ValidateSpec() error = %v, want errors.Is(%v)", err, tc.wantErr)
			}
		})
	}
}

func TestResolveBoundsShouldTreatAdministrativeCeilingsAsInclusive(t *testing.T) {
	t.Parallel()

	limits := participation.Limits{
		MaxWakes:          64,
		MaxWakeWallTime:   "15m",
		MaxTotalWallTime:  "2h",
		MaxInputTokens:    1_000_000,
		MaxOutputTokens:   200_000,
		MaxWakeDepth:      5,
		MinCoalesceWindow: "100ms",
		MaxCoalesceWindow: "5s",
	}
	equal := 64
	resolved, err := participation.ResolveBounds(
		&participation.BoundsRequest{MaxWakes: &equal},
		completeBounds(),
		limits,
	)
	if err != nil {
		t.Fatalf("ResolveBounds(equal ceiling) error = %v", err)
	}
	if resolved.MaxWakes != equal {
		t.Fatalf("ResolveBounds(equal ceiling).MaxWakes = %d, want %d", resolved.MaxWakes, equal)
	}

	above := 65
	_, err = participation.ResolveBounds(
		&participation.BoundsRequest{MaxWakes: &above},
		completeBounds(),
		limits,
	)
	if !errors.Is(err, participation.ErrBoundsExceedCeiling) {
		t.Fatalf("ResolveBounds(above ceiling) error = %v, want ErrBoundsExceedCeiling", err)
	}
	if !strings.Contains(err.Error(), "network.live.limits.max_wakes") {
		t.Fatalf("ResolveBounds(above ceiling) error = %q, want ceiling key", err)
	}

	unsafeDefaults := completeBounds()
	unsafeDefaults.MaxInputTokens = 9_007_199_254_740_992
	if _, err := participation.ResolveBounds(nil, unsafeDefaults, participation.Limits{}); !errors.Is(
		err,
		participation.ErrBoundsExceedCeiling,
	) {
		t.Fatalf("ResolveBounds(unsafe token default) error = %v, want ErrBoundsExceedCeiling", err)
	}

	unsafeLimits := limits
	unsafeLimits.MaxOutputTokens = 9_007_199_254_740_992
	if err := unsafeLimits.Validate(); !errors.Is(err, participation.ErrBoundsExceedCeiling) {
		t.Fatalf("Limits.Validate(unsafe token ceiling) error = %v, want ErrBoundsExceedCeiling", err)
	}
}

type participationResolverStub struct{}

func (participationResolverStub) Resolve(
	context.Context,
	participation.ResolveInput,
) (participation.Spec, error) {
	return participation.LocalSpec(), nil
}

type participationObserverStub struct {
	observation participation.ResolvedObservation
	err         error
}

func (*participationObserverStub) Resolve(
	context.Context,
	participation.ResolveInput,
) (participation.Spec, error) {
	return participation.LocalSpec(), nil
}

func (s *participationObserverStub) ObserveParticipationResolved(
	_ context.Context,
	observation participation.ResolvedObservation,
) error {
	s.observation = observation
	return s.err
}

func newTestResolver(t *testing.T, options participation.ResolverOptions) participation.Resolver {
	t.Helper()
	options.Defaults = completeBounds()
	options.Limits = participation.Limits{
		MaxWakes:          64,
		MaxWakeWallTime:   "15m",
		MaxTotalWallTime:  "2h",
		MaxInputTokens:    1_000_000,
		MaxOutputTokens:   200_000,
		MaxWakeDepth:      5,
		MinCoalesceWindow: "100ms",
		MaxCoalesceWindow: "5s",
	}
	resolver, err := participation.NewResolver(options)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	return resolver
}

func completeBoundsRequest() *participation.BoundsRequest {
	bounds := completeBounds()
	return &participation.BoundsRequest{
		MaxWakes:         new(bounds.MaxWakes),
		MaxWakeWallTime:  new(bounds.MaxWakeWallTime),
		MaxTotalWallTime: new(bounds.MaxTotalWallTime),
		MaxInputTokens:   new(bounds.MaxInputTokens),
		MaxOutputTokens:  new(bounds.MaxOutputTokens),
		MaxWakeDepth:     new(bounds.MaxWakeDepth),
		CoalesceWindow:   new(bounds.CoalesceWindow),
	}
}

func completeBounds() participation.Bounds {
	return participation.Bounds{
		MaxWakes:         8,
		MaxWakeWallTime:  "5m",
		MaxTotalWallTime: "30m",
		MaxInputTokens:   200_000,
		MaxOutputTokens:  50_000,
		MaxWakeDepth:     3,
		CoalesceWindow:   "500ms",
	}
}
