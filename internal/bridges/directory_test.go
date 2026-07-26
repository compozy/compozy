package bridges_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/testutil"
)

type bridgeTargetDirectoryStore struct {
	stubRegistryStore

	mu        sync.RWMutex
	refreshes map[string]time.Time
	targets   map[string]map[string]bridgepkg.BridgeTarget
}

func newBridgeTargetDirectoryStore() *bridgeTargetDirectoryStore {
	return &bridgeTargetDirectoryStore{
		stubRegistryStore: stubRegistryStore{
			getBridgeInstanceFn: func(_ context.Context, id string) (bridgepkg.BridgeInstance, error) {
				trimmedID := strings.TrimSpace(id)
				if trimmedID == "" {
					return bridgepkg.BridgeInstance{}, bridgepkg.ErrBridgeInstanceNotFound
				}
				return bridgepkg.BridgeInstance{
					ID:            trimmedID,
					Scope:         bridgepkg.ScopeGlobal,
					Platform:      "slack",
					ExtensionName: "slack-extension",
					DisplayName:   "Slack",
					Enabled:       true,
					Status:        bridgepkg.BridgeStatusReady,
					DMPolicy:      bridgepkg.BridgeDMPolicyOpen,
				}, nil
			},
		},
		refreshes: make(map[string]time.Time),
		targets:   make(map[string]map[string]bridgepkg.BridgeTarget),
	}
}

func (s *bridgeTargetDirectoryStore) RefreshBridgeTargets(
	_ context.Context,
	bridgeID string,
	targets []bridgepkg.BridgeTarget,
	refreshedAt time.Time,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	trimmedBridgeID := strings.TrimSpace(bridgeID)
	if trimmedBridgeID == "" {
		return errors.New("bridge id is required")
	}
	if s.targets[trimmedBridgeID] == nil {
		s.targets[trimmedBridgeID] = make(map[string]bridgepkg.BridgeTarget)
	}
	for _, target := range targets {
		s.targets[trimmedBridgeID][target.CanonicalRoute] = cloneBridgeTargetForDirectoryTest(target)
	}
	s.refreshes[trimmedBridgeID] = refreshedAt.UTC()
	return nil
}

func (s *bridgeTargetDirectoryStore) ListBridgeTargets(
	_ context.Context,
	query bridgepkg.BridgeTargetQuery,
) (bridgepkg.BridgeTargetPage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bridgeID := strings.TrimSpace(query.BridgeID)
	items := s.bridgeTargetsLocked(bridgeID)
	if strings.TrimSpace(query.Query) != "" {
		filter := bridgepkg.NormalizeBridgeTargetName(query.Query)
		items = slices.DeleteFunc(items, func(target bridgepkg.BridgeTarget) bool {
			return !strings.Contains(target.Normalized, filter) &&
				!strings.Contains(target.Qualifier, filter) &&
				!strings.Contains(strings.ToLower(target.CanonicalRoute), filter)
		})
	}
	total := len(items)
	if query.Limit > 0 && len(items) > query.Limit {
		items = items[:query.Limit]
	}
	return bridgepkg.BridgeTargetPage{
		Items:                   cloneBridgeTargetsForDirectoryTest(items),
		Total:                   total,
		LastSuccessfulRefreshAt: s.refreshes[bridgeID],
	}, nil
}

func (s *bridgeTargetDirectoryStore) GetBridgeTargetByCanonical(
	_ context.Context,
	bridgeID string,
	canonicalRoute string,
) (bridgepkg.BridgeTarget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	target, ok := s.targets[strings.TrimSpace(bridgeID)][strings.TrimSpace(canonicalRoute)]
	if !ok {
		return bridgepkg.BridgeTarget{}, bridgepkg.ErrBridgeTargetUnknown
	}
	return cloneBridgeTargetForDirectoryTest(target), nil
}

func (s *bridgeTargetDirectoryStore) FindBridgeTargetsByNormalized(
	_ context.Context,
	bridgeID string,
	normalized string,
) ([]bridgepkg.BridgeTarget, error) {
	want := bridgepkg.NormalizeBridgeTargetName(normalized)
	return s.findBridgeTargets(bridgeID, func(target bridgepkg.BridgeTarget) bool {
		return target.Normalized == want
	}), nil
}

func (s *bridgeTargetDirectoryStore) FindBridgeTargetsByQualifiedName(
	_ context.Context,
	bridgeID string,
	qualifier string,
	normalized string,
) ([]bridgepkg.BridgeTarget, error) {
	wantQualifier := bridgepkg.NormalizeBridgeTargetQualifier(qualifier)
	wantName := bridgepkg.NormalizeBridgeTargetName(normalized)
	return s.findBridgeTargets(bridgeID, func(target bridgepkg.BridgeTarget) bool {
		return target.Qualifier == wantQualifier && target.Normalized == wantName
	}), nil
}

func (s *bridgeTargetDirectoryStore) FindBridgeTargetsByPrefix(
	_ context.Context,
	bridgeID string,
	normalizedPrefix string,
) ([]bridgepkg.BridgeTarget, error) {
	prefix := bridgepkg.NormalizeBridgeTargetName(normalizedPrefix)
	return s.findBridgeTargets(bridgeID, func(target bridgepkg.BridgeTarget) bool {
		return strings.HasPrefix(target.Normalized, prefix)
	}), nil
}

func (s *bridgeTargetDirectoryStore) findBridgeTargets(
	bridgeID string,
	matches func(bridgepkg.BridgeTarget) bool,
) []bridgepkg.BridgeTarget {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var found []bridgepkg.BridgeTarget
	for _, target := range s.bridgeTargetsLocked(strings.TrimSpace(bridgeID)) {
		if matches(target) {
			found = append(found, target)
		}
	}
	return cloneBridgeTargetsForDirectoryTest(found)
}

func (s *bridgeTargetDirectoryStore) bridgeTargetsLocked(bridgeID string) []bridgepkg.BridgeTarget {
	values := s.targets[bridgeID]
	items := make([]bridgepkg.BridgeTarget, 0, len(values))
	for _, target := range values {
		items = append(items, cloneBridgeTargetForDirectoryTest(target))
	}
	slices.SortFunc(items, func(left, right bridgepkg.BridgeTarget) int {
		if byName := strings.Compare(left.Normalized, right.Normalized); byName != 0 {
			return byName
		}
		if byQualifier := strings.Compare(left.Qualifier, right.Qualifier); byQualifier != 0 {
			return byQualifier
		}
		return strings.Compare(left.CanonicalRoute, right.CanonicalRoute)
	})
	return items
}

func TestBridgeTargetDirectoryShouldResolveWithFourStepAlgorithm(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve exact canonical names, qualified names, prefixes, and ambiguity", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
		store := newBridgeTargetDirectoryStore()
		registry := bridgepkg.NewRegistry(store, bridgepkg.WithNow(func() time.Time { return base }))
		_, err := registry.RefreshBridgeTargets(testutil.Context(t), "brg-slack", []bridgepkg.BridgeTargetSnapshot{
			{
				CanonicalRoute: "slack://T1/C1",
				DisplayName:    "General",
				TargetType:     bridgepkg.BridgeTargetTypeChannel,
				Qualifier:      "workspace-a",
				Capabilities:   []string{"send", "thread"},
			},
			{
				CanonicalRoute: "slack://T2/C2",
				DisplayName:    "General",
				TargetType:     bridgepkg.BridgeTargetTypeChannel,
				Qualifier:      "workspace-b",
				Capabilities:   []string{"send"},
			},
			{
				CanonicalRoute: "slack://T1/C3",
				DisplayName:    "Ops Escalations",
				TargetType:     bridgepkg.BridgeTargetTypeRoom,
				Qualifier:      "workspace-a",
				Capabilities:   []string{"send"},
			},
		})
		if err != nil {
			t.Fatalf("RefreshBridgeTargets() error = %v", err)
		}

		testCases := []struct {
			name             string
			query            string
			wantStep         int
			wantRoute        string
			wantErr          error
			wantCandidateLen int
		}{
			{
				name:      "Should resolve exact canonical route at step one",
				query:     "slack://T1/C3",
				wantStep:  1,
				wantRoute: "slack://T1/C3",
			},
			{
				name:      "Should resolve exact normalized display name at step two",
				query:     "Ops Escalations",
				wantStep:  2,
				wantRoute: "slack://T1/C3",
			},
			{
				name:      "Should resolve qualified friendly name at step three",
				query:     "workspace-a/general",
				wantStep:  3,
				wantRoute: "slack://T1/C1",
			},
			{
				name:      "Should resolve a unique normalized prefix at step four",
				query:     "Ops",
				wantStep:  4,
				wantRoute: "slack://T1/C3",
			},
			{
				name:             "Should reject exact normalized ambiguity without picking",
				query:            "General",
				wantStep:         2,
				wantErr:          bridgepkg.ErrBridgeTargetAmbiguous,
				wantCandidateLen: 2,
			},
			{
				name:             "Should reject prefix ambiguity without picking",
				query:            "Gen",
				wantStep:         4,
				wantErr:          bridgepkg.ErrBridgeTargetAmbiguous,
				wantCandidateLen: 2,
			},
			{
				name:    "Should report unknown targets explicitly",
				query:   "finance",
				wantErr: bridgepkg.ErrBridgeTargetUnknown,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				result, resolveErr := registry.ResolveBridgeTarget(testutil.Context(t), "brg-slack", tc.query)
				if tc.wantErr != nil {
					if !errors.Is(resolveErr, tc.wantErr) {
						t.Fatalf("ResolveBridgeTarget() error = %v, want %v", resolveErr, tc.wantErr)
					}
					if result.Match != nil {
						t.Fatalf("ResolveBridgeTarget().Match = %#v, want nil for failed lookup", result.Match)
					}
					if got := result.Step; got != tc.wantStep {
						t.Fatalf("ResolveBridgeTarget().Step = %d, want %d", got, tc.wantStep)
					}
					if got := len(result.Candidates); got != tc.wantCandidateLen {
						t.Fatalf("len(ResolveBridgeTarget().Candidates) = %d, want %d", got, tc.wantCandidateLen)
					}
					return
				}
				if resolveErr != nil {
					t.Fatalf("ResolveBridgeTarget() error = %v", resolveErr)
				}
				if result.Match == nil {
					t.Fatal("ResolveBridgeTarget().Match = nil, want target")
				}
				if got := result.Step; got != tc.wantStep {
					t.Fatalf("ResolveBridgeTarget().Step = %d, want %d", got, tc.wantStep)
				}
				if got := result.Match.CanonicalRoute; got != tc.wantRoute {
					t.Fatalf("ResolveBridgeTarget().Match.CanonicalRoute = %q, want %q", got, tc.wantRoute)
				}
			})
		}
	})
}

func TestBridgeTargetDirectoryShouldListCacheFreshness(t *testing.T) {
	t.Parallel()

	t.Run("Should report stale cache from bridge-level refresh timestamp", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
		store := newBridgeTargetDirectoryStore()
		registry := bridgepkg.NewRegistry(store, bridgepkg.WithNow(func() time.Time { return now }))
		_, err := registry.RefreshBridgeTargets(testutil.Context(t), "brg-slack", []bridgepkg.BridgeTargetSnapshot{
			{
				CanonicalRoute: "slack://T1/C1",
				DisplayName:    "General",
				TargetType:     bridgepkg.BridgeTargetTypeChannel,
			},
		})
		if err != nil {
			t.Fatalf("RefreshBridgeTargets() error = %v", err)
		}

		now = now.Add(7 * time.Hour)
		result, err := registry.ListBridgeTargets(testutil.Context(t), bridgepkg.BridgeTargetQuery{
			BridgeID: "brg-slack",
		})
		if err != nil {
			t.Fatalf("ListBridgeTargets() error = %v", err)
		}
		if !result.CacheStale {
			t.Fatal("ListBridgeTargets().CacheStale = false, want true")
		}
		if got, want := result.Total, 1; got != want {
			t.Fatalf("ListBridgeTargets().Total = %d, want %d", got, want)
		}
	})
}

func cloneBridgeTargetsForDirectoryTest(values []bridgepkg.BridgeTarget) []bridgepkg.BridgeTarget {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]bridgepkg.BridgeTarget, len(values))
	for index, value := range values {
		cloned[index] = cloneBridgeTargetForDirectoryTest(value)
	}
	return cloned
}

func cloneBridgeTargetForDirectoryTest(value bridgepkg.BridgeTarget) bridgepkg.BridgeTarget {
	cloned := value
	cloned.Capabilities = slices.Clone(value.Capabilities)
	return cloned
}
