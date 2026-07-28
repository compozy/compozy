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
		return nil, fmt.Errorf("%w: Loop Run definition digest is required", ErrValidation)
	}
	snapshot, err := r.store.GetLoopDefinitionSnapshot(ctx, run.WorkspaceID, digest)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(snapshot.Digest) != digest {
		return nil, fmt.Errorf("%w: Loop definition snapshot digest changed", ErrValidation)
	}
	resolved, err := LoadExecutedDefinitionSnapshot(snapshot.Definition, digest)
	if err != nil {
		return nil, err
	}
	if resolved.DefinitionVersion != snapshot.Version {
		return nil, fmt.Errorf("%w: Loop definition snapshot version changed", ErrValidation)
	}
	return resolved, nil
}
