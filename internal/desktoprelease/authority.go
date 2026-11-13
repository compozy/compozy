package desktoprelease

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
)

func (a *Authority) Publish(ctx context.Context, request PublishRequest) (OperatorResult, error) {
	if err := validateAuthorityRequest(
		request.OperationID,
		request.Channel,
		request.Version,
		request.PublishedAt,
	); err != nil {
		return OperatorResult{}, err
	}
	branch := ChannelBranchPrefix + request.Channel
	if existing, err := a.backend.FindOperation(ctx, branch, request.OperationID); err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	} else if existing != nil {
		if err := validateExistingOperation(existing, operationPublish, request.Version); err != nil {
			return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
		}
		verified, verifyErr := a.verifyRemoteInventory(ctx, *existing)
		if verifyErr != nil {
			return OperatorResult{}, verifyErr
		}
		return resultFromExisting(operationPublish, existing, verified), nil
	}
	before, previous, err := a.channelState(ctx, branch)
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	if previous.Generation.Version != "" {
		if err := AssertStrictlyGreater(request.Version, previous.Generation.Version); err != nil {
			return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
		}
	}
	compatibility, err := ReadCompatibility(filepath.Join(request.AssetDir, CompatibilityFile))
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorInventoryIncomplete, err)
	}
	if compatibility.RuntimeVersion != request.Version {
		return OperatorResult{}, errorWithCode(
			ErrorVerificationFailed,
			fmt.Errorf(
				"desktop release: compat runtime_version %s does not match release %s",
				compatibility.RuntimeVersion,
				request.Version,
			),
		)
	}
	if previous.Generation.Version != "" {
		if err := AssertCompatibleWithPrevious(compatibility.MinAppVersion, previous.Generation.Version); err != nil {
			return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
		}
	}
	verified, err := a.publishAndVerifyInventory(ctx, request.AssetDir, request.Version)
	if err != nil {
		return OperatorResult{}, err
	}
	generation := Generation{
		OperationID: request.OperationID, Operation: operationPublish, Version: request.Version,
		MinAppVersion: compatibility.MinAppVersion, PublishedAt: request.PublishedAt.UTC(),
	}
	files, err := prepareChannelFiles(request.ChannelDir, request.AssetDir, a.backend.Repository(), generation)
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	auditCommit, err := a.backend.Commit(ctx, CommitRequest{ParentSHA: before, Generation: generation, Files: files})
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	if err := a.backend.CompareAndSwapRef(ctx, branch, before, auditCommit); err != nil {
		if errors.Is(err, ErrCompareAndSwap) {
			return OperatorResult{}, errorWithCode(ErrorChannelCASConflict, err)
		}
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	return OperatorResult{
		Operation: operationPublish, ChannelRefBefore: before, ChannelRefAfter: auditCommit,
		VerifiedInventory: verified, AuditCommit: auditCommit, Outcome: "published",
	}, nil
}

func (a *Authority) Repair(ctx context.Context, request RepairRequest) (OperatorResult, error) {
	if err := validateAuthorityRequest(
		request.OperationID,
		request.Channel,
		request.Version,
		request.PublishedAt,
	); err != nil {
		return OperatorResult{}, err
	}
	branch := ChannelBranchPrefix + request.Channel
	if existing, err := a.backend.FindOperation(ctx, branch, request.OperationID); err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	} else if existing != nil {
		if err := validateExistingOperation(existing, operationRepair, request.Version); err != nil {
			return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
		}
		verified, verifyErr := a.verifyRemoteInventory(ctx, *existing)
		if verifyErr != nil {
			return OperatorResult{}, verifyErr
		}
		return resultFromExisting(operationRepair, existing, verified), nil
	}
	before, current, err := a.channelState(ctx, branch)
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	knownGood, err := a.backend.KnownGood(ctx, branch, request.Version)
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	verified, err := a.verifyRemoteInventory(ctx, knownGood)
	if err != nil {
		return OperatorResult{}, err
	}
	generation := knownGood.Generation
	generation.OperationID = request.OperationID
	generation.Operation = operationRepair
	generation.PublishedAt = request.PublishedAt.UTC()
	generation.SourceCommit = knownGood.SHA
	files := cloneFiles(knownGood.Files)
	windowsManifest := path.Join(ChannelDirectory, ManifestWindows)
	if _, restored := files[windowsManifest]; !restored {
		if _, present := current.Files[windowsManifest]; present {
			// A pre-Windows rollback must not retain a feed from the replaced generation.
			files[windowsManifest] = nil
		}
	}
	generationBytes, err := canonicalJSON(generation)
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	files[path.Join(ChannelDirectory, GenerationFile)] = generationBytes
	auditCommit, err := a.backend.Commit(ctx, CommitRequest{ParentSHA: before, Generation: generation, Files: files})
	if err != nil {
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	if err := a.backend.CompareAndSwapRef(ctx, branch, before, auditCommit); err != nil {
		if errors.Is(err, ErrCompareAndSwap) {
			return OperatorResult{}, errorWithCode(ErrorChannelCASConflict, err)
		}
		return OperatorResult{}, errorWithCode(ErrorVerificationFailed, err)
	}
	return OperatorResult{
		Operation: operationRepair, ChannelRefBefore: before, ChannelRefAfter: auditCommit,
		VerifiedInventory: verified, AuditCommit: auditCommit, Outcome: "repaired",
	}, nil
}
