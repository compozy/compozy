//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package fileutil

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDirectoryRootedMutationsKeepTheHeldParentAfterPathReplacement(t *testing.T) {
	t.Parallel()

	t.Run("Should publish into the held directory after its path becomes a symlink", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		liveParent := filepath.Join(root, "live")
		archivedParent := filepath.Join(root, "archived")
		externalParent := filepath.Join(root, "external")
		if err := os.MkdirAll(liveParent, 0o700); err != nil {
			t.Fatalf("os.MkdirAll(live parent) error = %v", err)
		}
		if err := os.MkdirAll(externalParent, 0o700); err != nil {
			t.Fatalf("os.MkdirAll(external parent) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(liveParent, "target"), []byte("before"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(live target) error = %v", err)
		}
		sentinelPath := filepath.Join(externalParent, "target")
		sentinel := []byte("external sentinel")
		if err := os.WriteFile(sentinelPath, sentinel, 0o600); err != nil {
			t.Fatalf("os.WriteFile(external sentinel) error = %v", err)
		}

		directory, err := OpenDirectory(liveParent)
		if err != nil {
			t.Fatalf("OpenDirectory(live parent) error = %v", err)
		}
		defer func() {
			if closeErr := directory.Close(); closeErr != nil {
				t.Errorf("Directory.Close() error = %v", closeErr)
			}
		}()
		replaceDirectoryPathWithExternalSymlink(t, liveParent, archivedParent, externalParent)

		want := []byte("updated through held parent")
		if err := directory.AtomicWriteFile("target", want, 0o600, true); err != nil {
			t.Fatalf("Directory.AtomicWriteFile() error = %v", err)
		}
		assertRootedSentinelUnchanged(t, sentinelPath, sentinel)
		got, err := os.ReadFile(filepath.Join(archivedParent, "target"))
		if err != nil {
			t.Fatalf("os.ReadFile(archived target) error = %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("archived target = %q, want %q", got, want)
		}
	})

	t.Run("Should remove only the child below the held directory after path replacement", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		liveParent := filepath.Join(root, "live")
		archivedParent := filepath.Join(root, "archived")
		externalParent := filepath.Join(root, "external")
		if err := os.MkdirAll(filepath.Join(liveParent, "agent", "nested"), 0o700); err != nil {
			t.Fatalf("os.MkdirAll(live agent) error = %v", err)
		}
		if err := os.MkdirAll(filepath.Join(externalParent, "agent"), 0o700); err != nil {
			t.Fatalf("os.MkdirAll(external agent) error = %v", err)
		}
		liveStatePath := filepath.Join(liveParent, "agent", "nested", "state")
		if err := os.WriteFile(liveStatePath, []byte("remove"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(live state) error = %v", err)
		}
		sentinelPath := filepath.Join(externalParent, "agent", "sentinel")
		sentinel := []byte("keep external tree")
		if err := os.WriteFile(sentinelPath, sentinel, 0o600); err != nil {
			t.Fatalf("os.WriteFile(external sentinel) error = %v", err)
		}

		directory, err := OpenDirectory(liveParent)
		if err != nil {
			t.Fatalf("OpenDirectory(live parent) error = %v", err)
		}
		defer func() {
			if closeErr := directory.Close(); closeErr != nil {
				t.Errorf("Directory.Close() error = %v", closeErr)
			}
		}()
		replaceDirectoryPathWithExternalSymlink(t, liveParent, archivedParent, externalParent)

		if err := directory.RemoveAll("agent"); err != nil {
			t.Fatalf("Directory.RemoveAll() error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(archivedParent, "agent")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(archived agent) error = %v, want %v", err, os.ErrNotExist)
		}
		assertRootedSentinelUnchanged(t, sentinelPath, sentinel)
	})
}

// Invariant: concurrent no-replace directory publishers yield one committed target and one typed collision.
// Owner: fileutil capability-rooted filesystem boundary.
// Canonical suite: fileutil rooted-write filesystem unit tests.
func TestDirectoryMoveToNoReplace(t *testing.T) {
	t.Parallel()

	t.Run("Should atomically reject one concurrent no-replace publisher", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
			t.Skip("atomic no-replace directory moves require a supported platform primitive")
		}

		rootPath := t.TempDir()
		for _, source := range []string{"first", "second"} {
			if err := os.Mkdir(filepath.Join(rootPath, source), 0o700); err != nil {
				t.Fatalf("os.Mkdir(%q) error = %v", source, err)
			}
			if err := os.WriteFile(filepath.Join(rootPath, source, "winner"), []byte(source), 0o600); err != nil {
				t.Fatalf("os.WriteFile(%q) error = %v", source, err)
			}
		}

		root, err := OpenDirectory(rootPath)
		if err != nil {
			t.Fatalf("OpenDirectory(root) error = %v", err)
		}
		defer func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("Directory.Close(root) error = %v", closeErr)
			}
		}()

		start := make(chan struct{})
		type moveOutcome struct {
			result DirectoryMoveResult
			err    error
		}
		results := make(chan moveOutcome, 2)
		for _, source := range []string{"first", "second"} {
			go func(source string) {
				<-start
				moveSource, err := root.OpenDirectoryForMove(source)
				if err != nil {
					results <- moveOutcome{err: err}
					return
				}
				result, moveErr := moveSource.MoveTo(root, "target", false)
				closeErr := moveSource.Close()
				results <- moveOutcome{result: result, err: errors.Join(moveErr, result.PostCommitErr, closeErr)}
			}(source)
		}
		close(start)

		var successCount int
		var collisionCount int
		for range 2 {
			outcome := <-results
			switch {
			case outcome.result.Committed() && outcome.err == nil:
				successCount++
			case outcome.result.Collision() && errors.Is(outcome.err, ErrTargetExists):
				collisionCount++
			default:
				t.Fatalf(
					"MoveTo() result = %+v, error = %v, want committed or %v collision",
					outcome.result,
					outcome.err,
					ErrTargetExists,
				)
			}
		}
		if successCount != 1 || collisionCount != 1 {
			t.Fatalf("MoveTo() successes = %d, collisions = %d, want one each", successCount, collisionCount)
		}

		winner, err := os.ReadFile(filepath.Join(rootPath, "target", "winner"))
		if err != nil {
			t.Fatalf("os.ReadFile(target winner) error = %v", err)
		}
		if string(winner) != "first" && string(winner) != "second" {
			t.Fatalf("target winner = %q, want first or second", winner)
		}
	})
}

// Invariant: a directory move publishes only the identity held by its source
// capability, and every result reports whether the canonical target changed.
// Owner: fileutil capability-rooted directory publication.
// Canonical suite: fileutil rooted-write filesystem unit tests.
func TestDirectoryMoveToBindsTheHeldSourceIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should publish the held directory and report a committed result", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		if err := os.Mkdir(filepath.Join(rootPath, "source"), 0o700); err != nil {
			t.Fatalf("os.Mkdir(source) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(rootPath, "source", "payload"), []byte("held"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(source payload) error = %v", err)
		}

		root, err := OpenOrCreateDirectory(rootPath, 0o700)
		if err != nil {
			t.Fatalf("OpenOrCreateDirectory(root) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("Directory.Close(root) error = %v", closeErr)
			}
		})
		source, err := root.OpenDirectoryForMove("source")
		if err != nil {
			t.Fatalf("OpenDirectoryForMove(source) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := source.Close(); closeErr != nil {
				t.Errorf("Directory.Close(source) error = %v", closeErr)
			}
		})

		result, err := source.MoveTo(root, "target", false)
		if err != nil {
			t.Fatalf("Directory.MoveTo() error = %v", err)
		}
		if !result.Committed() || result.Collision() || result.PostCommitErr != nil {
			t.Fatalf("Directory.MoveTo() result = %+v, want committed without post-commit error", result)
		}
		got, err := os.ReadFile(filepath.Join(rootPath, "target", "payload"))
		if err != nil {
			t.Fatalf("os.ReadFile(target payload) error = %v", err)
		}
		if string(got) != "held" {
			t.Fatalf("target payload = %q, want held", got)
		}
		if _, err := os.Stat(filepath.Join(rootPath, "source")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(source) error = %v, want %v", err, os.ErrNotExist)
		}
	})

	t.Run("Should restore the source and report a collision without replacing the target", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		for _, directoryName := range []string{"source", "target"} {
			if err := os.Mkdir(filepath.Join(rootPath, directoryName), 0o700); err != nil {
				t.Fatalf("os.Mkdir(%q) error = %v", directoryName, err)
			}
		}
		if err := os.WriteFile(filepath.Join(rootPath, "target", "payload"), []byte("existing"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(target payload) error = %v", err)
		}

		root, err := OpenOrCreateDirectory(rootPath, 0o700)
		if err != nil {
			t.Fatalf("OpenOrCreateDirectory(root) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("Directory.Close(root) error = %v", closeErr)
			}
		})
		source, err := root.OpenDirectoryForMove("source")
		if err != nil {
			t.Fatalf("OpenDirectoryForMove(source) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := source.Close(); closeErr != nil {
				t.Errorf("Directory.Close(source) error = %v", closeErr)
			}
		})

		result, err := source.MoveTo(root, "target", false)
		if !result.Collision() || result.Committed() || result.PostCommitErr != nil {
			t.Fatalf("Directory.MoveTo() result = %+v, want collision without post-commit error", result)
		}
		if !errors.Is(err, ErrTargetExists) {
			t.Fatalf("Directory.MoveTo() error = %v, want ErrTargetExists", err)
		}
		if _, err := os.Stat(filepath.Join(rootPath, "source")); err != nil {
			t.Fatalf("os.Stat(source) error = %v, want source restored", err)
		}
		got, err := os.ReadFile(filepath.Join(rootPath, "target", "payload"))
		if err != nil {
			t.Fatalf("os.ReadFile(target payload) error = %v", err)
		}
		if string(got) != "existing" {
			t.Fatalf("target payload = %q, want existing", got)
		}
	})

	t.Run("Should preserve the retained directory when a collision also blocks rollback", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		for _, name := range []string{"source", "target"} {
			if err := os.Mkdir(filepath.Join(rootPath, name), 0o700); err != nil {
				t.Fatalf("os.Mkdir(%q) error = %v", name, err)
			}
		}
		if err := os.WriteFile(filepath.Join(rootPath, "source", "payload"), []byte("held"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(source payload) error = %v", err)
		}

		root := openMoveTestRoot(t, rootPath)
		source := openMoveTestDirectory(t, root, "source")
		move, err := prepareDirectoryMoveCandidate(source)
		if err != nil {
			t.Fatalf("prepareDirectoryMoveCandidate() error = %v", err)
		}
		if err := os.Mkdir(filepath.Join(rootPath, "source"), 0o700); err != nil {
			t.Fatalf("os.Mkdir(replacement source) error = %v", err)
		}

		result, err := commitPreparedDirectoryMove(source.move, root, "target", false, move)
		if !result.Collision() || result.Committed() {
			t.Fatalf("commitPreparedDirectoryMove() result = %+v, want collision", result)
		}
		if !errors.Is(err, ErrTargetExists) || !errors.Is(err, ErrDirectoryMoveRecoveryRequired) {
			t.Fatalf("commitPreparedDirectoryMove() error = %v, want collision and recovery required", err)
		}
		assertDirectoryMoveRecoveryEntry(t, rootPath, err, "held")
		if _, statErr := os.Stat(filepath.Join(rootPath, "source")); statErr != nil {
			t.Fatalf("os.Stat(replacement source) error = %v", statErr)
		}
	})

	t.Run("Should restore the exact retained directory after a candidate-name substitution", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		for _, name := range []string{"source", "target"} {
			if err := os.Mkdir(filepath.Join(rootPath, name), 0o700); err != nil {
				t.Fatalf("os.Mkdir(%q) error = %v", name, err)
			}
		}
		if err := os.WriteFile(filepath.Join(rootPath, "source", "payload"), []byte("held"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(source payload) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(rootPath, "target", "payload"), []byte("existing"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(target payload) error = %v", err)
		}

		root := openMoveTestRoot(t, rootPath)
		source := openMoveTestDirectory(t, root, "source")
		move, err := prepareDirectoryMoveCandidate(source)
		if err != nil {
			t.Fatalf("prepareDirectoryMoveCandidate() error = %v", err)
		}
		transactionPath := filepath.Join(rootPath, move.transactionName)
		if err := os.Rename(
			filepath.Join(transactionPath, directoryMoveCandidateName),
			filepath.Join(transactionPath, "retained"),
		); err != nil {
			t.Fatalf("os.Rename(retained candidate) error = %v", err)
		}
		if err := os.Mkdir(filepath.Join(transactionPath, directoryMoveCandidateName), 0o700); err != nil {
			t.Fatalf("os.Mkdir(substitute candidate) error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(transactionPath, directoryMoveCandidateName, "payload"),
			[]byte("substitute"),
			0o600,
		); err != nil {
			t.Fatalf("os.WriteFile(substitute payload) error = %v", err)
		}

		result, err := commitPreparedDirectoryMove(source.move, root, "target", false, move)
		if !result.Collision() || result.Committed() {
			t.Fatalf("commitPreparedDirectoryMove() result = %+v, want collision", result)
		}
		if !errors.Is(err, ErrTargetExists) || errors.Is(err, ErrDirectoryMoveRecoveryRequired) {
			t.Fatalf("commitPreparedDirectoryMove() error = %v, want recovered collision", err)
		}
		assertRegularFileContents(t, filepath.Join(rootPath, "source", "payload"), "held")
		assertRegularFileContents(t, filepath.Join(rootPath, "target", "payload"), "existing")
		if _, statErr := os.Stat(transactionPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(private transaction) error = %v, want removal", statErr)
		}
	})

	t.Run("Should detect a post-verification directory substitution and retain the exact entry", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		if err := os.Mkdir(filepath.Join(rootPath, "source"), 0o700); err != nil {
			t.Fatalf("os.Mkdir(source) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(rootPath, "source", "payload"), []byte("held"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(source payload) error = %v", err)
		}

		root := openMoveTestRoot(t, rootPath)
		source := openMoveTestDirectory(t, root, "source")
		move, err := prepareDirectoryMoveCandidate(source)
		if err != nil {
			t.Fatalf("prepareDirectoryMoveCandidate() error = %v", err)
		}
		transactionPath := filepath.Join(rootPath, move.transactionName)
		if err := os.Rename(
			filepath.Join(transactionPath, directoryMoveCandidateName),
			filepath.Join(transactionPath, "retained"),
		); err != nil {
			t.Fatalf("os.Rename(retained candidate) error = %v", err)
		}
		if err := os.Mkdir(filepath.Join(transactionPath, directoryMoveCandidateName), 0o700); err != nil {
			t.Fatalf("os.Mkdir(substitute candidate) error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(transactionPath, directoryMoveCandidateName, "payload"),
			[]byte("substitute"),
			0o600,
		); err != nil {
			t.Fatalf("os.WriteFile(substitute payload) error = %v", err)
		}

		result, err := commitPreparedDirectoryMove(source.move, root, "target", false, move)
		if !result.Committed() {
			t.Fatalf("commitPreparedDirectoryMove() result = %+v, want target mutation reported", result)
		}
		if !errors.Is(err, ErrMoveSourceIdentityChanged) || !errors.Is(err, ErrDirectoryMoveRecoveryRequired) {
			t.Fatalf("commitPreparedDirectoryMove() error = %v, want identity change and recovery required", err)
		}
		assertRegularFileContents(t, filepath.Join(rootPath, "target", "payload"), "substitute")
		assertDirectoryMoveRecoveryEntry(t, rootPath, err, "held")
	})

	t.Run("Should detect a post-verification directory removal substitution and preserve the retained tree", func(
		t *testing.T,
	) {
		t.Parallel()

		rootPath := t.TempDir()
		if err := os.Mkdir(filepath.Join(rootPath, "source"), 0o700); err != nil {
			t.Fatalf("os.Mkdir(source) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(rootPath, "source", "payload"), nil, 0o600); err != nil {
			t.Fatalf("os.WriteFile(source payload) error = %v", err)
		}
		root := openMoveTestRoot(t, rootPath)
		source := openMoveTestDirectory(t, root, "source")
		transactionName, transaction, err := root.CreateTemporaryDirectory(".compozy-remove-", 0o700)
		if err != nil {
			t.Fatalf("CreateTemporaryDirectory() error = %v", err)
		}
		result, err := source.MoveTo(transaction, boundDirectoryRemovalCandidate, false)
		if err != nil || !result.Committed() || result.PostCommitErr != nil {
			t.Fatalf("Directory.MoveTo(removal transaction) result = %+v, error = %v", result, err)
		}
		transactionPath := filepath.Join(rootPath, transactionName)
		if err := os.Rename(
			filepath.Join(transactionPath, boundDirectoryRemovalCandidate),
			filepath.Join(transactionPath, "retained"),
		); err != nil {
			t.Fatalf("os.Rename(retained removal candidate) error = %v", err)
		}
		if err := os.Mkdir(filepath.Join(transactionPath, boundDirectoryRemovalCandidate), 0o700); err != nil {
			t.Fatalf("os.Mkdir(substitute removal candidate) error = %v", err)
		}

		removed, removeErr := removeBoundEmptyDirectory(source, transaction)
		if removed || !errors.Is(removeErr, ErrMoveSourceIdentityChanged) {
			t.Fatalf("removeBoundEmptyDirectory() removed = %t, error = %v, want identity refusal", removed, removeErr)
		}
		restoreErr := restoreBoundRemovalDirectory(source, root, "source")
		recoveryErr := finishFailedBoundRemoval(
			root,
			transactionName,
			transaction,
			source.move.identity,
			restoreErr,
		)
		err = errors.Join(removeErr, restoreErr, recoveryErr)
		if !errors.Is(err, ErrDirectoryMoveRecoveryRequired) {
			t.Fatalf("bound removal recovery error = %v, want recovery required", err)
		}
		assertDirectoryMoveRecoveryEntry(t, rootPath, err, "")
	})

	t.Run("Should reject a replacement source before changing the canonical target", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		sourcePath := filepath.Join(rootPath, "source")
		archivedPath := filepath.Join(rootPath, "archived")
		if err := os.Mkdir(sourcePath, 0o700); err != nil {
			t.Fatalf("os.Mkdir(source) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(sourcePath, "payload"), []byte("held"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(source payload) error = %v", err)
		}

		root, err := OpenOrCreateDirectory(rootPath, 0o700)
		if err != nil {
			t.Fatalf("OpenOrCreateDirectory(root) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("Directory.Close(root) error = %v", closeErr)
			}
		})
		source, err := root.OpenDirectoryForMove("source")
		if err != nil {
			t.Fatalf("OpenDirectoryForMove(source) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := source.Close(); closeErr != nil {
				t.Errorf("Directory.Close(source) error = %v", closeErr)
			}
		})

		if err := os.Rename(sourcePath, archivedPath); err != nil {
			t.Fatalf("os.Rename(source to archived) error = %v", err)
		}
		if err := os.Mkdir(sourcePath, 0o700); err != nil {
			t.Fatalf("os.Mkdir(replacement source) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(sourcePath, "payload"), []byte("substitute"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(replacement payload) error = %v", err)
		}

		result, err := source.MoveTo(root, "target", false)
		if result.Committed() || result.Collision() || result.PostCommitErr != nil {
			t.Fatalf("Directory.MoveTo() result = %+v, want pre-commit failure", result)
		}
		if !errors.Is(err, ErrMoveSourceIdentityChanged) {
			t.Fatalf("Directory.MoveTo() error = %v, want ErrMoveSourceIdentityChanged", err)
		}
		if _, err := os.Stat(filepath.Join(rootPath, "target")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(target) error = %v, want %v", err, os.ErrNotExist)
		}
		replacement, err := os.ReadFile(filepath.Join(rootPath, "source", "payload"))
		if err != nil {
			t.Fatalf("os.ReadFile(replacement source payload) error = %v", err)
		}
		if string(replacement) != "substitute" {
			t.Fatalf("replacement source payload = %q, want substitute", replacement)
		}
		archived, err := os.ReadFile(filepath.Join(archivedPath, "payload"))
		if err != nil {
			t.Fatalf("os.ReadFile(archived source payload) error = %v", err)
		}
		if string(archived) != "held" {
			t.Fatalf("archived source payload = %q, want held", archived)
		}
	})

	t.Run("Should remove the exact directory through its committed move capability", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		sourcePath := filepath.Join(rootPath, "source")
		if err := os.Mkdir(sourcePath, 0o700); err != nil {
			t.Fatalf("os.Mkdir(source) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(sourcePath, "payload"), []byte("held"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(source payload) error = %v", err)
		}

		root, err := OpenOrCreateDirectory(rootPath, 0o700)
		if err != nil {
			t.Fatalf("OpenOrCreateDirectory(root) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("Directory.Close(root) error = %v", closeErr)
			}
		})
		source, err := root.OpenDirectoryForMove("source")
		if err != nil {
			t.Fatalf("OpenDirectoryForMove(source) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := source.Close(); closeErr != nil {
				t.Errorf("Directory.Close(source) error = %v", closeErr)
			}
		})

		result, err := source.MoveTo(root, "target", false)
		if err != nil || !result.Committed() || result.PostCommitErr != nil {
			t.Fatalf("Directory.MoveTo(target) result = %+v, error = %v", result, err)
		}
		if err := source.RemoveBoundTree(); err != nil {
			t.Fatalf("Directory.RemoveBoundTree() error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(rootPath, "target")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(target) error = %v, want %v", err, os.ErrNotExist)
		}
	})

	t.Run("Should refuse to remove a replacement at the committed name", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		sourcePath := filepath.Join(rootPath, "source")
		targetPath := filepath.Join(rootPath, "target")
		archivedPath := filepath.Join(rootPath, "archived")
		if err := os.Mkdir(sourcePath, 0o700); err != nil {
			t.Fatalf("os.Mkdir(source) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(sourcePath, "payload"), []byte("held"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(source payload) error = %v", err)
		}

		root, err := OpenOrCreateDirectory(rootPath, 0o700)
		if err != nil {
			t.Fatalf("OpenOrCreateDirectory(root) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("Directory.Close(root) error = %v", closeErr)
			}
		})
		source, err := root.OpenDirectoryForMove("source")
		if err != nil {
			t.Fatalf("OpenDirectoryForMove(source) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := source.Close(); closeErr != nil {
				t.Errorf("Directory.Close(source) error = %v", closeErr)
			}
		})

		result, err := source.MoveTo(root, "target", false)
		if err != nil || !result.Committed() || result.PostCommitErr != nil {
			t.Fatalf("Directory.MoveTo(target) result = %+v, error = %v", result, err)
		}
		if err := os.Rename(targetPath, archivedPath); err != nil {
			t.Fatalf("os.Rename(target to archived) error = %v", err)
		}
		if err := os.Mkdir(targetPath, 0o700); err != nil {
			t.Fatalf("os.Mkdir(replacement target) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(targetPath, "payload"), []byte("substitute"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(replacement payload) error = %v", err)
		}

		err = source.RemoveBoundTree()
		if !errors.Is(err, ErrMoveSourceIdentityChanged) {
			t.Fatalf("Directory.RemoveBoundTree() error = %v, want identity change", err)
		}
		for path, want := range map[string]string{
			archivedPath: "held",
			targetPath:   "substitute",
		} {
			got, readErr := os.ReadFile(filepath.Join(path, "payload"))
			if readErr != nil {
				t.Fatalf("os.ReadFile(%q) error = %v", path, readErr)
			}
			if string(got) != want {
				t.Fatalf("payload at %q = %q, want %q", path, got, want)
			}
		}
	})
}

// Invariant: regular-file move and removal operations mutate only the inode
// retained by the supplied source handle.
// Owner: fileutil capability-rooted regular-file mutation boundary.
// Canonical suite: fileutil rooted-write filesystem unit tests.
func TestRegularFileMutationBindsTheHeldSourceIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a substituted move source without changing either file", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		sourcePath := filepath.Join(rootPath, "source")
		archivedPath := filepath.Join(rootPath, "archived")
		if err := os.WriteFile(sourcePath, []byte("held"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(source) error = %v", err)
		}
		root, err := OpenOrCreateDirectory(rootPath, 0o700)
		if err != nil {
			t.Fatalf("OpenOrCreateDirectory(root) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("Directory.Close(root) error = %v", closeErr)
			}
		})
		source, err := root.OpenRegularFile("source")
		if err != nil {
			t.Fatalf("Directory.OpenRegularFile(source) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := source.Close(); closeErr != nil {
				t.Errorf("os.File.Close(source) error = %v", closeErr)
			}
		})

		if err := os.Rename(sourcePath, archivedPath); err != nil {
			t.Fatalf("os.Rename(source to archived) error = %v", err)
		}
		if err := os.WriteFile(sourcePath, []byte("substitute"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(substitute) error = %v", err)
		}

		committed, err := root.MoveRegularFileNoReplace(source, "source", "target")
		if committed {
			t.Fatal("MoveRegularFileNoReplace() committed = true, want false")
		}
		if !errors.Is(err, ErrMoveSourceIdentityChanged) {
			t.Fatalf("MoveRegularFileNoReplace() error = %v, want ErrMoveSourceIdentityChanged", err)
		}
		assertRegularFileContents(t, sourcePath, "substitute")
		assertRegularFileContents(t, archivedPath, "held")
		if _, statErr := os.Stat(filepath.Join(rootPath, "target")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(target) error = %v, want os.ErrNotExist", statErr)
		}
	})

	t.Run("Should reject a substituted removal source without deleting either file", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		sourcePath := filepath.Join(rootPath, "source")
		archivedPath := filepath.Join(rootPath, "archived")
		if err := os.WriteFile(sourcePath, []byte("held"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(source) error = %v", err)
		}
		root, err := OpenOrCreateDirectory(rootPath, 0o700)
		if err != nil {
			t.Fatalf("OpenOrCreateDirectory(root) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("Directory.Close(root) error = %v", closeErr)
			}
		})
		source, err := root.OpenRegularFile("source")
		if err != nil {
			t.Fatalf("Directory.OpenRegularFile(source) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := source.Close(); closeErr != nil {
				t.Errorf("os.File.Close(source) error = %v", closeErr)
			}
		})

		if err := os.Rename(sourcePath, archivedPath); err != nil {
			t.Fatalf("os.Rename(source to archived) error = %v", err)
		}
		if err := os.WriteFile(sourcePath, []byte("substitute"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(substitute) error = %v", err)
		}

		err = root.RemoveBoundRegularFile(source, "source")
		if !errors.Is(err, ErrMoveSourceIdentityChanged) {
			t.Fatalf("RemoveBoundRegularFile() error = %v, want ErrMoveSourceIdentityChanged", err)
		}
		assertRegularFileContents(t, sourcePath, "substitute")
		assertRegularFileContents(t, archivedPath, "held")
	})

	t.Run("Should preserve the retained file when a collision also blocks rollback", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		sourcePath := filepath.Join(rootPath, "source")
		if err := os.WriteFile(sourcePath, []byte("held"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(source) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(rootPath, "target"), []byte("existing"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(target) error = %v", err)
		}
		root := openMoveTestRoot(t, rootPath)
		source, sourceInfo := openMoveTestRegularFile(t, root, "source")
		move, err := prepareRegularFileMoveCandidate(root, source, sourceInfo, "source")
		if err != nil {
			t.Fatalf("prepareRegularFileMoveCandidate() error = %v", err)
		}
		if err := os.WriteFile(sourcePath, []byte("replacement"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(replacement source) error = %v", err)
		}

		committed, err := commitPreparedRegularFileMove(root, sourceInfo, "source", "target", move)
		if committed {
			t.Fatal("commitPreparedRegularFileMove() committed = true, want false")
		}
		if !errors.Is(err, ErrTargetExists) || !errors.Is(err, ErrDirectoryMoveRecoveryRequired) {
			t.Fatalf("commitPreparedRegularFileMove() error = %v, want collision and recovery required", err)
		}
		assertRegularFileMoveRecoveryEntry(t, rootPath, err, "held")
		assertRegularFileContents(t, sourcePath, "replacement")
	})

	t.Run("Should restore the exact retained file after a candidate-name substitution", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		sourcePath := filepath.Join(rootPath, "source")
		targetPath := filepath.Join(rootPath, "target")
		if err := os.WriteFile(sourcePath, []byte("held"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(source) error = %v", err)
		}
		if err := os.WriteFile(targetPath, []byte("existing"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(target) error = %v", err)
		}
		root := openMoveTestRoot(t, rootPath)
		source, sourceInfo := openMoveTestRegularFile(t, root, "source")
		move, err := prepareRegularFileMoveCandidate(root, source, sourceInfo, "source")
		if err != nil {
			t.Fatalf("prepareRegularFileMoveCandidate() error = %v", err)
		}
		transactionPath := filepath.Join(rootPath, move.transactionName)
		if err := os.Rename(
			filepath.Join(transactionPath, regularFileMoveCandidateName),
			filepath.Join(transactionPath, "retained"),
		); err != nil {
			t.Fatalf("os.Rename(retained candidate) error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(transactionPath, regularFileMoveCandidateName),
			[]byte("substitute"),
			0o600,
		); err != nil {
			t.Fatalf("os.WriteFile(substitute candidate) error = %v", err)
		}

		committed, err := commitPreparedRegularFileMove(root, sourceInfo, "source", "target", move)
		if committed {
			t.Fatal("commitPreparedRegularFileMove() committed = true, want false")
		}
		if !errors.Is(err, ErrTargetExists) || errors.Is(err, ErrDirectoryMoveRecoveryRequired) {
			t.Fatalf("commitPreparedRegularFileMove() error = %v, want recovered collision", err)
		}
		assertRegularFileContents(t, sourcePath, "held")
		assertRegularFileContents(t, targetPath, "existing")
		if _, statErr := os.Stat(transactionPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(private transaction) error = %v, want removal", statErr)
		}
	})

	t.Run("Should detect a post-verification file substitution and retain the exact entry", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		if err := os.WriteFile(filepath.Join(rootPath, "source"), []byte("held"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(source) error = %v", err)
		}
		root := openMoveTestRoot(t, rootPath)
		source, sourceInfo := openMoveTestRegularFile(t, root, "source")
		move, err := prepareRegularFileMoveCandidate(root, source, sourceInfo, "source")
		if err != nil {
			t.Fatalf("prepareRegularFileMoveCandidate() error = %v", err)
		}
		transactionPath := filepath.Join(rootPath, move.transactionName)
		if err := os.Rename(
			filepath.Join(transactionPath, regularFileMoveCandidateName),
			filepath.Join(transactionPath, "retained"),
		); err != nil {
			t.Fatalf("os.Rename(retained candidate) error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(transactionPath, regularFileMoveCandidateName),
			[]byte("substitute"),
			0o600,
		); err != nil {
			t.Fatalf("os.WriteFile(substitute candidate) error = %v", err)
		}

		committed, err := commitPreparedRegularFileMove(root, sourceInfo, "source", "target", move)
		if !committed {
			t.Fatal("commitPreparedRegularFileMove() committed = false, want target mutation reported")
		}
		if !errors.Is(err, ErrMoveSourceIdentityChanged) || !errors.Is(err, ErrDirectoryMoveRecoveryRequired) {
			t.Fatalf("commitPreparedRegularFileMove() error = %v, want identity change and recovery required", err)
		}
		assertRegularFileContents(t, filepath.Join(rootPath, "target"), "substitute")
		assertRegularFileMoveRecoveryEntry(t, rootPath, err, "held")
	})

	t.Run("Should detect a post-verification removal substitution and preserve the retained file", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		if err := os.WriteFile(filepath.Join(rootPath, "source"), []byte("held"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(source) error = %v", err)
		}
		root := openMoveTestRoot(t, rootPath)
		source, sourceInfo := openMoveTestRegularFile(t, root, "source")
		move, err := prepareRegularFileMoveCandidate(root, source, sourceInfo, "source")
		if err != nil {
			t.Fatalf("prepareRegularFileMoveCandidate() error = %v", err)
		}
		transactionPath := filepath.Join(rootPath, move.transactionName)
		if err := os.Rename(
			filepath.Join(transactionPath, regularFileMoveCandidateName),
			filepath.Join(transactionPath, "retained"),
		); err != nil {
			t.Fatalf("os.Rename(retained candidate) error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(transactionPath, regularFileMoveCandidateName),
			[]byte("substitute"),
			0o600,
		); err != nil {
			t.Fatalf("os.WriteFile(substitute candidate) error = %v", err)
		}

		err = removePreparedRegularFile(root, sourceInfo, move)
		if !errors.Is(err, ErrMoveSourceIdentityChanged) || !errors.Is(err, ErrDirectoryMoveRecoveryRequired) {
			t.Fatalf("removePreparedRegularFile() error = %v, want identity change and recovery required", err)
		}
		assertRegularFileMoveRecoveryEntry(t, rootPath, err, "held")
	})
}

func openMoveTestRoot(t *testing.T, path string) *Directory {
	t.Helper()

	root, err := OpenOrCreateDirectory(path, 0o700)
	if err != nil {
		t.Fatalf("OpenOrCreateDirectory(%q) error = %v", path, err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("Directory.Close(root) error = %v", err)
		}
	})
	return root
}

func openMoveTestDirectory(t *testing.T, root *Directory, name string) *Directory {
	t.Helper()

	directory, err := root.OpenDirectoryForMove(name)
	if err != nil {
		t.Fatalf("OpenDirectoryForMove(%q) error = %v", name, err)
	}
	t.Cleanup(func() {
		if err := directory.Close(); err != nil {
			t.Errorf("Directory.Close(%q) error = %v", name, err)
		}
	})
	return directory
}

func openMoveTestRegularFile(t *testing.T, root *Directory, name string) (*os.File, os.FileInfo) {
	t.Helper()

	file, err := root.OpenRegularFile(name)
	if err != nil {
		t.Fatalf("OpenRegularFile(%q) error = %v", name, err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("os.File.Close(%q) error = %v", name, err)
		}
	})
	info, err := file.Stat()
	if err != nil {
		t.Fatalf("os.File.Stat(%q) error = %v", name, err)
	}
	return file, info
}

func assertDirectoryMoveRecoveryEntry(t *testing.T, rootPath string, err error, wantPayload string) {
	t.Helper()

	recovery, ok := errors.AsType[*MoveRecoveryError](err)
	if !ok {
		t.Fatalf("error = %v, want *MoveRecoveryError", err)
	}
	payloadPath := filepath.Join(rootPath, recovery.TransactionName, recovery.CandidateName, "payload")
	assertRegularFileContents(t, payloadPath, wantPayload)
}

func assertRegularFileMoveRecoveryEntry(t *testing.T, rootPath string, err error, want string) {
	t.Helper()

	recovery, ok := errors.AsType[*MoveRecoveryError](err)
	if !ok {
		t.Fatalf("error = %v, want *MoveRecoveryError", err)
	}
	assertRegularFileContents(t, filepath.Join(rootPath, recovery.TransactionName, recovery.CandidateName), want)
}

func assertRegularFileContents(t *testing.T, path string, want string) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("file %q = %q, want %q", path, got, want)
	}
}

func replaceDirectoryPathWithExternalSymlink(t *testing.T, live string, archived string, external string) {
	t.Helper()

	if err := os.Rename(live, archived); err != nil {
		t.Fatalf("os.Rename(live parent) error = %v", err)
	}
	if err := os.Symlink(external, live); err != nil {
		t.Fatalf("os.Symlink(external parent) error = %v", err)
	}
}

func assertRootedSentinelUnchanged(t *testing.T, path string, want []byte) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(external sentinel) error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("external sentinel = %q, want %q", got, want)
	}
}
