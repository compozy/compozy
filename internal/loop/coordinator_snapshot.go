package loop

import (
	"context"
	"fmt"
	"strings"
)

func (r *CoordinatorRunner) resolvePinnedDefinition(
	ctx context.Context,
	run Run,
) (*ResolvedDefinition, error) {
	digest := strings.TrimSpace(run.DefinitionDigest)
	if digest == "" {
		return nil, fmt.Errorf(
			"%w: %w: Loop Run definition digest is required",
			ErrExecutedDefinitionSnapshot,
			ErrValidation,
		)
	}
	snapshot, err := r.store.GetLoopDefinitionSnapshot(ctx, run.WorkspaceID, digest)
	if err != nil {
		return nil, fmt.Errorf("%w: load digest %q: %w", ErrExecutedDefinitionSnapshot, digest, err)
	}
	if strings.TrimSpace(snapshot.Digest) != digest {
		return nil, fmt.Errorf(
			"%w: %w: Loop definition snapshot digest changed",
			ErrExecutedDefinitionSnapshot,
			ErrValidation,
		)
	}
	resolved, err := LoadExecutedDefinitionSnapshot(snapshot.Definition, digest)
	if err != nil {
		return nil, fmt.Errorf("%w: hydrate digest %q: %w", ErrExecutedDefinitionSnapshot, digest, err)
	}
	if resolved.DefinitionVersion != snapshot.Version {
		return nil, fmt.Errorf(
			"%w: %w: Loop definition snapshot version changed",
			ErrExecutedDefinitionSnapshot,
			ErrValidation,
		)
	}
	return resolved, nil
}
