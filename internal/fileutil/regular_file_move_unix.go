//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package fileutil

import (
	"errors"
	"fmt"
	"os"
)

const regularFileMoveCandidateName = "candidate"

type regularFileMoveCandidate struct {
	transactionName  string
	transaction      *Directory
	candidate        *os.File
	rollbackIdentity os.FileInfo
}

func moveRegularFileNoReplace(
	directory *Directory,
	source *os.File,
	sourceInfo os.FileInfo,
	sourceName string,
	targetName string,
) (bool, error) {
	move, err := prepareRegularFileMoveCandidate(directory, source, sourceInfo, sourceName)
	if err != nil {
		return false, err
	}
	return commitPreparedRegularFileMove(directory, sourceInfo, sourceName, targetName, move)
}

func commitPreparedRegularFileMove(
	directory *Directory,
	sourceInfo os.FileInfo,
	sourceName string,
	targetName string,
	move *regularFileMoveCandidate,
) (bool, error) {
	if err := moveDirectoryNoFollow(
		move.transaction.file,
		move.candidate,
		regularFileMoveCandidateName,
		directory.file,
		targetName,
		false,
	); err != nil {
		return false, recoverRegularFileMove(directory, sourceName, move, err)
	}
	target, targetInfo, err := directory.openRegularFile(targetName)
	if err != nil {
		return true, preserveRegularFileMove(
			directory,
			sourceInfo,
			move,
			errors.Join(ErrMoveSourceIdentityChanged, fmt.Errorf("fileutil: open moved regular-file target: %w", err)),
		)
	}
	targetCloseErr := target.Close()
	if !os.SameFile(sourceInfo, targetInfo) {
		return true, preserveRegularFileMove(directory, sourceInfo, move, errors.Join(
			ErrMoveSourceIdentityChanged,
			targetCloseErr,
		))
	}
	if targetCloseErr != nil {
		return true, preserveRegularFileMove(directory, sourceInfo, move, targetCloseErr)
	}

	return true, finishCommittedRegularFileMove(directory, sourceName, targetName, move)
}

func removeBoundRegularFile(
	directory *Directory,
	source *os.File,
	sourceInfo os.FileInfo,
	sourceName string,
) error {
	move, err := prepareRegularFileMoveCandidate(directory, source, sourceInfo, sourceName)
	if err != nil {
		return err
	}
	return removePreparedRegularFile(directory, sourceInfo, move)
}

func removePreparedRegularFile(
	directory *Directory,
	sourceInfo os.FileInfo,
	move *regularFileMoveCandidate,
) error {
	beforeLinks, err := unixFileLinkCount(move.candidate)
	if err != nil {
		return preserveRegularFileMove(directory, sourceInfo, move, err)
	}
	if err := removeNoFollowAt(move.transaction.file, regularFileMoveCandidateName, false); err != nil {
		return preserveRegularFileMove(directory, sourceInfo, move, err)
	}
	afterLinks, err := unixFileLinkCount(move.candidate)
	if err != nil {
		return preserveRegularFileMove(directory, sourceInfo, move, err)
	}
	if afterLinks >= beforeLinks {
		return preserveRegularFileMove(directory, sourceInfo, move, ErrMoveSourceIdentityChanged)
	}

	candidateCloseErr := move.candidate.Close()
	transactionCloseErr := move.transaction.Close()
	removeTransactionErr := directory.RemoveAll(move.transactionName)
	syncErr := directory.Sync()
	return errors.Join(
		wrapDirectoryMovePostCommitError("close removed regular-file candidate", candidateCloseErr),
		wrapDirectoryMovePostCommitError("close regular-file removal transaction", transactionCloseErr),
		wrapDirectoryMovePostCommitError("remove regular-file removal transaction", removeTransactionErr),
		wrapDirectoryMovePostCommitError("sync regular-file removal parent", syncErr),
	)
}

func prepareRegularFileMoveCandidate(
	directory *Directory,
	source *os.File,
	sourceInfo os.FileInfo,
	sourceName string,
) (*regularFileMoveCandidate, error) {
	transactionName, transaction, err := directory.CreateTemporaryDirectory(".compozy-file-move-", 0o700)
	if err != nil {
		return nil, fmt.Errorf("fileutil: create private regular-file move directory: %w", err)
	}
	if err := moveDirectoryNoFollow(
		directory.file,
		source,
		sourceName,
		transaction.file,
		regularFileMoveCandidateName,
		false,
	); err != nil {
		return nil, closeEmptyMoveTransaction(directory, transactionName, transaction, err)
	}

	candidate, candidateInfo, err := transaction.openRegularFile(regularFileMoveCandidateName)
	move := &regularFileMoveCandidate{
		transactionName:  transactionName,
		transaction:      transaction,
		candidate:        candidate,
		rollbackIdentity: candidateInfo,
	}
	if err != nil {
		return nil, recoverRegularFileMove(directory, sourceName, move, err)
	}
	if !os.SameFile(sourceInfo, candidateInfo) {
		return nil, recoverRegularFileMove(directory, sourceName, move, ErrMoveSourceIdentityChanged)
	}
	return move, nil
}

func recoverRegularFileMove(
	directory *Directory,
	sourceName string,
	move *regularFileMoveCandidate,
	cause error,
) error {
	recoveryName, locateErr := locateRegularFileMoveRecoveryEntry(move.transaction, move.rollbackIdentity)
	var restoreErr error
	if recoveryName == "" {
		restoreErr = errors.Join(ErrDirectoryMoveRecoveryRequired, locateErr)
	} else {
		restoreErr = moveDirectoryNoFollow(
			move.transaction.file,
			move.candidate,
			recoveryName,
			directory.file,
			sourceName,
			false,
		)
		if restoreErr == nil {
			restoreErr = requireRegularFileIdentityAt(directory, sourceName, move.rollbackIdentity)
		}
	}
	candidateCloseErr := error(nil)
	if move.candidate != nil {
		candidateCloseErr = move.candidate.Close()
	}
	if restoreErr != nil {
		retainedName, retainedErr := locateRegularFileMoveRecoveryEntry(move.transaction, move.rollbackIdentity)
		if retainedName != "" {
			recoveryName = retainedName
		}
		transactionCloseErr := move.transaction.Close()
		preserveSyncErr := directory.Sync()
		return errors.Join(
			cause,
			&MoveRecoveryError{
				TransactionName: move.transactionName,
				CandidateName:   recoveryName,
				Cause: errors.Join(
					fmt.Errorf("fileutil: restore exact regular-file source identity: %w", restoreErr),
					locateErr,
					retainedErr,
				),
			},
			wrapDirectoryMovePostCommitError("close private regular-file candidate", candidateCloseErr),
			wrapDirectoryMovePostCommitError("close preserved private regular-file transaction", transactionCloseErr),
			wrapDirectoryMovePostCommitError("sync preserved private regular-file parent", preserveSyncErr),
		)
	}
	transactionCloseErr := move.transaction.Close()
	removeTransactionErr := directory.RemoveAll(move.transactionName)
	return errors.Join(
		cause,
		locateErr,
		wrapDirectoryMovePostCommitError("close private regular-file candidate", candidateCloseErr),
		wrapDirectoryMovePostCommitError("close private regular-file transaction", transactionCloseErr),
		wrapDirectoryMovePostCommitError("remove private regular-file transaction", removeTransactionErr),
	)
}

func requireRegularFileIdentityAt(directory *Directory, name string, want os.FileInfo) (retErr error) {
	if want == nil {
		return ErrMoveSourceIdentityChanged
	}
	current, info, err := directory.openRegularFile(name)
	if err != nil {
		return fmt.Errorf("fileutil: open restored regular-file source %q: %w", name, err)
	}
	defer func() {
		if closeErr := current.Close(); closeErr != nil {
			retErr = errors.Join(
				retErr,
				fmt.Errorf("fileutil: close restored regular-file source %q: %w", name, closeErr),
			)
		}
	}()
	if !os.SameFile(want, info) {
		return ErrMoveSourceIdentityChanged
	}
	return nil
}

func preserveRegularFileMove(
	directory *Directory,
	sourceInfo os.FileInfo,
	move *regularFileMoveCandidate,
	cause error,
) error {
	recoveryName, recoveryErr := locateRegularFileMoveRecoveryEntry(move.transaction, sourceInfo)
	candidateCloseErr := move.candidate.Close()
	transactionCloseErr := move.transaction.Close()
	syncErr := directory.Sync()
	return errors.Join(
		&MoveRecoveryError{
			TransactionName: move.transactionName,
			CandidateName:   recoveryName,
			Cause:           errors.Join(cause, recoveryErr),
		},
		wrapDirectoryMovePostCommitError("close unverified regular-file candidate", candidateCloseErr),
		wrapDirectoryMovePostCommitError("close preserved regular-file transaction", transactionCloseErr),
		wrapDirectoryMovePostCommitError("sync preserved regular-file parent", syncErr),
	)
}

func finishCommittedRegularFileMove(
	directory *Directory,
	sourceName string,
	targetName string,
	move *regularFileMoveCandidate,
) error {
	candidateCloseErr := move.candidate.Close()
	transactionCloseErr := move.transaction.Close()
	removeTransactionErr := directory.RemoveAll(move.transactionName)
	syncErr := directory.Sync()
	return errors.Join(
		wrapDirectoryMovePostCommitError("close moved regular-file candidate", candidateCloseErr),
		wrapDirectoryMovePostCommitError("close private regular-file transaction", transactionCloseErr),
		wrapDirectoryMovePostCommitError("remove private regular-file transaction", removeTransactionErr),
		wrapDirectoryMovePostCommitError(
			fmt.Sprintf("sync directory after moving regular file %q to %q", sourceName, targetName),
			syncErr,
		),
	)
}
