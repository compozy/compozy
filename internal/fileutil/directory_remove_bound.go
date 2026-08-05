package fileutil

import (
	"errors"
	"fmt"
)

const boundDirectoryRemovalCandidate = "candidate"

// RemoveBoundTree removes the exact directory held by a move capability. The
// directory is first moved into a private parent, so a replacement at its
// public name cannot be traversed or deleted.
func (d *Directory) RemoveBoundTree() error {
	if d == nil || d.file == nil || d.move == nil || d.move.parent == nil {
		return fmt.Errorf("%w: bound directory is closed", ErrInvalidPath)
	}

	originalParent := d.move.parent
	originalName := d.move.name
	originalIdentity := d.move.identity
	transactionName, transaction, err := originalParent.CreateTemporaryDirectory(".compozy-remove-", 0o700)
	if err != nil {
		return fmt.Errorf("fileutil: create bound removal transaction: %w", err)
	}

	result, moveErr := d.MoveTo(transaction, boundDirectoryRemovalCandidate, false)
	if !result.Committed() {
		return errors.Join(
			moveErr,
			result.PostCommitErr,
			closeBoundRemovalTransaction(originalParent, transactionName, transaction),
		)
	}
	committedMoveErr := errors.Join(moveErr, result.PostCommitErr)

	if removeErr := d.removeContents(); removeErr != nil {
		restoreErr := restoreBoundRemovalDirectory(d, originalParent, originalName)
		return errors.Join(
			committedMoveErr,
			fmt.Errorf("fileutil: remove bound directory contents: %w", removeErr),
			restoreErr,
			finishFailedBoundRemoval(
				originalParent,
				transactionName,
				transaction,
				originalIdentity,
				restoreErr,
			),
		)
	}

	removed, removeErr := removeBoundEmptyDirectory(d, transaction)
	if !removed {
		restoreErr := restoreBoundRemovalDirectory(d, originalParent, originalName)
		return errors.Join(
			committedMoveErr,
			fmt.Errorf("fileutil: remove empty bound directory: %w", removeErr),
			restoreErr,
			finishFailedBoundRemoval(
				originalParent,
				transactionName,
				transaction,
				originalIdentity,
				restoreErr,
			),
		)
	}
	d.move = nil
	directoryCloseErr := d.Close()

	return errors.Join(
		committedMoveErr,
		removeErr,
		wrapDirectoryMovePostCommitError("close removed bound directory", directoryCloseErr),
		closeBoundRemovalTransaction(originalParent, transactionName, transaction),
	)
}

func finishFailedBoundRemoval(
	parent *Directory,
	name string,
	transaction *Directory,
	identity directoryMoveIdentity,
	restoreErr error,
) error {
	if !errors.Is(restoreErr, ErrDirectoryMoveRecoveryRequired) {
		return closeBoundRemovalTransaction(parent, name, transaction)
	}
	recoveryName, recoveryErr := locateDirectoryMoveRecoveryEntry(transaction, identity)
	return errors.Join(
		&MoveRecoveryError{
			TransactionName: name,
			CandidateName:   recoveryName,
			Cause:           errors.Join(restoreErr, recoveryErr),
		},
		wrapDirectoryMovePostCommitError("close preserved bound removal transaction", transaction.Close()),
		wrapDirectoryMovePostCommitError("sync preserved bound removal parent", parent.Sync()),
	)
}

func restoreBoundRemovalDirectory(
	directory *Directory,
	parent *Directory,
	name string,
) error {
	result, err := directory.MoveTo(parent, name, false)
	if result.Committed() {
		return errors.Join(err, result.PostCommitErr)
	}
	return errors.Join(
		ErrDirectoryMoveRecoveryRequired,
		err,
		result.PostCommitErr,
	)
}

func closeBoundRemovalTransaction(parent *Directory, name string, transaction *Directory) error {
	closeErr := transaction.Close()
	removeErr := parent.RemoveAll(name)
	return errors.Join(
		wrapDirectoryMovePostCommitError("close bound removal transaction", closeErr),
		wrapDirectoryMovePostCommitError("remove bound removal transaction", removeErr),
	)
}
