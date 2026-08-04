package loop

import (
	"context"
	"fmt"
	"strings"
)

func (s *service) pinnedResolvedDefinition(
	ctx context.Context,
	run Run,
) (*ResolvedDefinition, error) {
	digest := strings.TrimSpace(run.DefinitionDigest)
	if digest == "" {
		return nil, fmt.Errorf("%w: executed definition digest is required", ErrValidation)
	}
	snapshot, err := s.store.GetLoopDefinitionSnapshot(ctx, run.WorkspaceID, digest)
	if err != nil {
		return nil, fmt.Errorf("load pinned Loop definition %q: %w", digest, err)
	}
	resolved, err := LoadExecutedDefinitionSnapshot(snapshot.Definition, digest)
	if err != nil {
		return nil, fmt.Errorf("load executed Loop definition %q: %w", digest, err)
	}
	return resolved, nil
}
