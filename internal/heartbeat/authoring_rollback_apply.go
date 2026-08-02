package heartbeat

import "context"

func (s *ManagedHeartbeatAuthoringService) applyAbsentRollback(
	ctx context.Context,
	target resolvedAuthoringTarget,
	current *ResolvedPolicy,
	actor AuthoringIdentity,
) (MutationResult, error) {
	if err := s.verifyUnchangedHeartbeat(ctx, target, current); err != nil {
		return MutationResult{}, err
	}
	snapshotID, revisionID, err := s.newPostMutationIDs()
	if err != nil {
		return MutationResult{}, err
	}
	if err := target.managedPath.Remove(); err != nil {
		return MutationResult{}, authoringDiagnosticError(
			diagnosticHeartbeatPathError,
			target.sourcePath,
			"HEARTBEAT.md could not be rolled back",
			err,
			ErrAuthoringPathRejected,
		)
	}
	return s.persistPostDelete(
		ctx,
		target,
		snapshotID,
		revisionID,
		current.Digest,
		RevisionOperationRollback,
		actor,
	)
}

func (s *ManagedHeartbeatAuthoringService) applyBodyRollback(
	ctx context.Context,
	target resolvedAuthoringTarget,
	current *ResolvedPolicy,
	body string,
	actor AuthoringIdentity,
) (MutationResult, error) {
	proposed, err := Parse(ctx, ParseRequest{
		SourcePath:    target.heartbeatPath,
		WorkspaceRoot: target.workspaceRoot,
		Content:       []byte(body),
		Config:        target.config,
	})
	if err != nil {
		return MutationResult{Policy: proposed}, authoringInvalidError(proposed.Diagnostics)
	}
	if err := s.verifyUnchangedHeartbeat(ctx, target, current); err != nil {
		return MutationResult{}, err
	}
	snapshotID, revisionID, err := s.newPostMutationIDs()
	if err != nil {
		return MutationResult{}, err
	}
	if err := target.managedPath.Write([]byte(body), s.mode); err != nil {
		return MutationResult{}, authoringDiagnosticError(
			diagnosticHeartbeatPathError,
			target.sourcePath,
			"HEARTBEAT.md could not be rolled back",
			err,
			ErrAuthoringPathRejected,
		)
	}
	return s.persistPostWrite(
		ctx,
		target,
		snapshotID,
		revisionID,
		current.Digest,
		RevisionOperationRollback,
		body,
		actor,
	)
}
