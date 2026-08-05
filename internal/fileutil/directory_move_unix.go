//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package fileutil

import (
	"errors"
	"fmt"
	"os"
)

const directoryMoveCandidateName = "candidate"

type directoryMoveCandidate struct {
	transactionName string
	transaction     *Directory
	candidate       *Directory
}

func moveBoundDirectory(
	source *Directory,
	target *Directory,
	targetName string,
	replace bool,
) (DirectoryMoveResult, error) {
	binding := source.move
	move, err := prepareDirectoryMoveCandidate(source)
	if err != nil {
		return DirectoryMoveResult{State: DirectoryMovePreCommit}, err
	}
	return commitPreparedDirectoryMove(binding, target, targetName, replace, move)
}

func commitPreparedDirectoryMove(
	binding *directoryMoveBinding,
	target *Directory,
	targetName string,
	replace bool,
	move *directoryMoveCandidate,
) (DirectoryMoveResult, error) {
	if err := moveDirectoryNoFollow(
		move.transaction.file,
		move.candidate.file,
		directoryMoveCandidateName,
		target.file,
		targetName,
		replace,
	); err != nil {
		state := DirectoryMovePreCommit
		if errors.Is(err, ErrTargetExists) {
			state = DirectoryMoveCollision
		}
		return DirectoryMoveResult{State: state}, recoverUncommittedDirectoryMove(
			binding,
			move.transactionName,
			move.transaction,
			move.candidate.file,
			move.candidate,
			err,
		)
	}
	if err := requireMovedDirectoryIdentity(binding.identity, target, targetName); err != nil {
		recoveryName, recoveryErr := locateDirectoryMoveRecoveryEntry(move.transaction, binding.identity)
		candidateCloseErr := move.candidate.Close()
		transactionCloseErr := move.transaction.Close()
		sourceSyncErr := binding.parent.Sync()
		targetSyncErr := error(nil)
		if target != binding.parent {
			targetSyncErr = target.Sync()
		}
		return DirectoryMoveResult{
				State: DirectoryMoveCommitted,
				PostCommitErr: errors.Join(
					wrapDirectoryMovePostCommitError("close unverified private candidate", candidateCloseErr),
					wrapDirectoryMovePostCommitError("close preserved private transaction", transactionCloseErr),
					wrapDirectoryMovePostCommitError("sync source parent", sourceSyncErr),
					wrapDirectoryMovePostCommitError("sync target parent", targetSyncErr),
				),
			}, errors.Join(
				&MoveRecoveryError{
					TransactionName: move.transactionName,
					CandidateName:   recoveryName,
					Cause:           errors.Join(err, recoveryErr),
				},
			)
	}

	candidateCloseErr := move.candidate.Close()
	transactionCloseErr := move.transaction.Close()
	removeTransactionErr := binding.parent.RemoveAll(move.transactionName)
	sourceSyncErr := binding.parent.Sync()
	targetSyncErr := error(nil)
	if target != binding.parent {
		targetSyncErr = target.Sync()
	}
	return DirectoryMoveResult{
		State: DirectoryMoveCommitted,
		PostCommitErr: errors.Join(
			wrapDirectoryMovePostCommitError("close private candidate", candidateCloseErr),
			wrapDirectoryMovePostCommitError("close private transaction", transactionCloseErr),
			wrapDirectoryMovePostCommitError("remove private transaction", removeTransactionErr),
			wrapDirectoryMovePostCommitError("sync source parent", sourceSyncErr),
			wrapDirectoryMovePostCommitError("sync target parent", targetSyncErr),
		),
	}, nil
}

func requireMovedDirectoryIdentity(
	want directoryMoveIdentity,
	target *Directory,
	targetName string,
) (retErr error) {
	current, err := target.OpenDirectory(targetName)
	if err != nil {
		return fmt.Errorf("fileutil: open moved directory target %q: %w", targetName, err)
	}
	defer func() {
		if closeErr := current.Close(); closeErr != nil {
			retErr = errors.Join(retErr, closeErr)
		}
	}()
	matches, err := directoryMoveIdentityMatches(want, current.file)
	if err != nil {
		return err
	}
	if !matches {
		return ErrMoveSourceIdentityChanged
	}
	return nil
}

func prepareDirectoryMoveCandidate(source *Directory) (*directoryMoveCandidate, error) {
	binding := source.move
	transactionName, transaction, err := binding.parent.CreateTemporaryDirectory(".compozy-move-", 0o700)
	if err != nil {
		return nil, fmt.Errorf("fileutil: create private move directory: %w", err)
	}

	if err := moveDirectoryNoFollow(
		binding.parent.file,
		source.file,
		binding.name,
		transaction.file,
		directoryMoveCandidateName,
		false,
	); err != nil {
		return nil, closeEmptyMoveTransaction(binding.parent, transactionName, transaction, err)
	}

	candidate, err := transaction.OpenDirectory(directoryMoveCandidateName)
	if err != nil {
		return nil, recoverUncommittedDirectoryMove(
			binding,
			transactionName,
			transaction,
			source.file,
			nil,
			err,
		)
	}

	matches, err := directoryMoveIdentityMatches(binding.identity, candidate.file)
	if err != nil {
		return nil, recoverUncommittedDirectoryMove(
			binding,
			transactionName,
			transaction,
			candidate.file,
			candidate,
			err,
		)
	}
	if !matches {
		return nil, recoverUncommittedDirectoryMove(
			binding,
			transactionName,
			transaction,
			candidate.file,
			candidate,
			ErrMoveSourceIdentityChanged,
		)
	}

	return &directoryMoveCandidate{
		transactionName: transactionName,
		transaction:     transaction,
		candidate:       candidate,
	}, nil
}

func closeEmptyMoveTransaction(parent *Directory, transactionName string, transaction *Directory, cause error) error {
	closeErr := transaction.Close()
	removeErr := parent.RemoveAll(transactionName)
	return errors.Join(
		cause,
		wrapDirectoryMovePostCommitError("close empty private transaction", closeErr),
		wrapDirectoryMovePostCommitError("remove empty private transaction", removeErr),
	)
}

func recoverUncommittedDirectoryMove(
	binding *directoryMoveBinding,
	transactionName string,
	transaction *Directory,
	sourceFile *os.File,
	candidate *Directory,
	cause error,
) error {
	rollbackIdentity, identityErr := directoryMoveIdentityFromFile(sourceFile)
	recoveryName, locateErr := locateDirectoryMoveRecoveryEntry(transaction, rollbackIdentity)
	var restoreErr error
	if identityErr != nil || recoveryName == "" {
		restoreErr = errors.Join(ErrDirectoryMoveRecoveryRequired, identityErr, locateErr)
	} else {
		restoreErr = moveDirectoryNoFollow(
			transaction.file,
			nil,
			recoveryName,
			binding.parent.file,
			binding.name,
			false,
		)
		if restoreErr == nil {
			restoreErr = requireMovedDirectoryIdentity(rollbackIdentity, binding.parent, binding.name)
		}
	}
	candidateCloseErr := error(nil)
	if candidate != nil {
		candidateCloseErr = candidate.Close()
	}
	if restoreErr != nil {
		retainedName, retainedErr := locateDirectoryMoveRecoveryEntry(transaction, rollbackIdentity)
		if retainedName != "" {
			recoveryName = retainedName
		}
		transactionCloseErr := transaction.Close()
		preserveSyncErr := binding.parent.Sync()
		return errors.Join(
			cause,
			&MoveRecoveryError{
				TransactionName: transactionName,
				CandidateName:   recoveryName,
				Cause: errors.Join(
					fmt.Errorf("restore exact source identity: %w", restoreErr),
					locateErr,
					retainedErr,
				),
			},
			wrapDirectoryMovePostCommitError("close private candidate", candidateCloseErr),
			wrapDirectoryMovePostCommitError("close preserved private transaction", transactionCloseErr),
			wrapDirectoryMovePostCommitError("sync preserved private transaction parent", preserveSyncErr),
		)
	}
	transactionCloseErr := transaction.Close()
	removeErr := binding.parent.RemoveAll(transactionName)
	return errors.Join(
		cause,
		locateErr,
		wrapDirectoryMovePostCommitError("close private candidate", candidateCloseErr),
		wrapDirectoryMovePostCommitError("close private transaction", transactionCloseErr),
		wrapDirectoryMovePostCommitError("remove private transaction", removeErr),
	)
}
