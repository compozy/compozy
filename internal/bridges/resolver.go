package bridges

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ResolveBridgeTargetResult reports the deterministic resolver step for one friendly target lookup.
type ResolveBridgeTargetResult struct {
	Match      *BridgeTarget  `json:"match,omitempty"`
	Step       int            `json:"step"`
	Ambiguous  bool           `json:"ambiguous"`
	Candidates []BridgeTarget `json:"candidates,omitempty"`
}

// ResolveBridgeTarget resolves a canonical bridge target using a deterministic 4-step algorithm.
func (s *Service) ResolveBridgeTarget(
	ctx context.Context,
	bridgeID string,
	query string,
) (ResolveBridgeTargetResult, error) {
	if err := s.checkReady(ctx, "resolve bridge target"); err != nil {
		return ResolveBridgeTargetResult{}, err
	}
	store, err := s.targetDirectoryStore()
	if err != nil {
		return ResolveBridgeTargetResult{}, err
	}
	trimmedBridgeID := strings.TrimSpace(bridgeID)
	if err := requireField(trimmedBridgeID, "bridge target bridge id"); err != nil {
		return ResolveBridgeTargetResult{}, err
	}
	if _, err := s.GetInstance(ctx, trimmedBridgeID); err != nil {
		return ResolveBridgeTargetResult{}, err
	}
	trimmedQuery := strings.TrimSpace(query)
	if err := requireField(trimmedQuery, "bridge target query"); err != nil {
		return ResolveBridgeTargetResult{}, err
	}

	result, resolved, err := resolveBridgeTargetByCanonical(ctx, store, trimmedBridgeID, trimmedQuery)
	if err != nil || resolved {
		return result, err
	}

	normalized := NormalizeBridgeTargetName(trimmedQuery)
	if err := requireField(normalized, "bridge target normalized query"); err != nil {
		return ResolveBridgeTargetResult{}, err
	}

	result, resolved, err = resolveBridgeTargetByNormalized(ctx, store, trimmedBridgeID, trimmedQuery, normalized)
	if err != nil || resolved {
		return result, err
	}

	result, resolved, err = resolveBridgeTargetByQualifiedName(ctx, store, trimmedBridgeID, trimmedQuery)
	if err != nil || resolved {
		return result, err
	}

	result, resolved, err = resolveBridgeTargetByPrefix(ctx, store, trimmedBridgeID, trimmedQuery, normalized)
	if err != nil || resolved {
		return result, err
	}

	return ResolveBridgeTargetResult{}, fmt.Errorf(
		"bridges: bridge target %q: %w",
		trimmedQuery,
		ErrBridgeTargetUnknown,
	)
}

func resolveBridgeTargetByCanonical(
	ctx context.Context,
	store TargetDirectoryStore,
	bridgeID string,
	query string,
) (ResolveBridgeTargetResult, bool, error) {
	target, err := store.GetBridgeTargetByCanonical(ctx, bridgeID, query)
	if err == nil {
		return matchedBridgeTargetResult(1, target), true, nil
	}
	if !errors.Is(err, ErrBridgeTargetUnknown) {
		return ResolveBridgeTargetResult{}, false, err
	}
	return ResolveBridgeTargetResult{}, false, nil
}

func resolveBridgeTargetByNormalized(
	ctx context.Context,
	store TargetDirectoryStore,
	bridgeID string,
	query string,
	normalized string,
) (ResolveBridgeTargetResult, bool, error) {
	targets, err := store.FindBridgeTargetsByNormalized(ctx, bridgeID, normalized)
	if err != nil {
		return ResolveBridgeTargetResult{}, false, err
	}
	return resolveBridgeTargetCandidates(2, query, "normalized", targets)
}

func resolveBridgeTargetByQualifiedName(
	ctx context.Context,
	store TargetDirectoryStore,
	bridgeID string,
	query string,
) (ResolveBridgeTargetResult, bool, error) {
	qualifier, name, ok := splitQualifiedBridgeTargetQuery(query)
	if !ok {
		return ResolveBridgeTargetResult{}, false, nil
	}
	targets, err := store.FindBridgeTargetsByQualifiedName(ctx, bridgeID, qualifier, name)
	if err != nil {
		return ResolveBridgeTargetResult{}, false, err
	}
	return resolveBridgeTargetCandidates(3, query, "qualified", targets)
}

func resolveBridgeTargetByPrefix(
	ctx context.Context,
	store TargetDirectoryStore,
	bridgeID string,
	query string,
	normalized string,
) (ResolveBridgeTargetResult, bool, error) {
	targets, err := store.FindBridgeTargetsByPrefix(ctx, bridgeID, normalized)
	if err != nil {
		return ResolveBridgeTargetResult{}, false, err
	}
	return resolveBridgeTargetCandidates(4, query, "prefix", targets)
}

func resolveBridgeTargetCandidates(
	step int,
	query string,
	label string,
	targets []BridgeTarget,
) (ResolveBridgeTargetResult, bool, error) {
	switch len(targets) {
	case 0:
		return ResolveBridgeTargetResult{}, false, nil
	case 1:
		return matchedBridgeTargetResult(step, targets[0]), true, nil
	default:
		return ambiguousBridgeTargetResult(step, targets), true, fmt.Errorf(
			"bridges: bridge target %q matched %d %s candidates: %w",
			query,
			len(targets),
			label,
			ErrBridgeTargetAmbiguous,
		)
	}
}

func matchedBridgeTargetResult(step int, target BridgeTarget) ResolveBridgeTargetResult {
	cloned := cloneBridgeTarget(target)
	return ResolveBridgeTargetResult{Match: &cloned, Step: step}
}

func ambiguousBridgeTargetResult(step int, targets []BridgeTarget) ResolveBridgeTargetResult {
	return ResolveBridgeTargetResult{
		Step:       step,
		Ambiguous:  true,
		Candidates: cloneBridgeTargets(targets),
	}
}

func splitQualifiedBridgeTargetQuery(query string) (string, string, bool) {
	left, right, ok := strings.Cut(strings.TrimSpace(query), "/")
	if !ok {
		return "", "", false
	}
	qualifier := NormalizeBridgeTargetQualifier(left)
	name := NormalizeBridgeTargetName(right)
	if qualifier == "" || name == "" {
		return "", "", false
	}
	return qualifier, name, true
}
