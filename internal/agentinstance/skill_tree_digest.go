package agentinstance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

// ErrSkillTreeUnsupported reports a skill tree with a symlink or another non-regular entry.
var ErrSkillTreeUnsupported = errors.New("skill_tree_unsupported")

// errSkillTreeChanged reports a tree that changed while it was digested.
var errSkillTreeChanged = errors.New("skill tree changed while it was read")

const skillTreeDigestHeader = "compozy-skill-tree-v1\n"
const skillUnsupportedMode = fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky

type skillContextReader struct {
	ctx    context.Context
	source io.Reader
}

func (r skillContextReader) Read(buf []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.source.Read(buf)
}

// SkillTreeDigest returns "sha256:<hex>" over the v1 manifest (ADR-002):
//
//	"compozy-skill-tree-v1\n" then, for each regular file sorted by path bytes,
//	"<len(path)>:<path>\n<perm octal>\n<size>\n<sha256(content) hex>\n".
//
// Paths are slash-separated and relative to dir. Directories contribute only through their files.
// Symlinks, non-regular entries and files with special mode bits return ErrSkillTreeUnsupported.
func SkillTreeDigest(dir string) (string, error) {
	return digestSkillTree(context.Background(), dir)
}

func digestSkillTree(ctx context.Context, dir string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: %s is not a directory", ErrSkillTreeUnsupported, filepath.Base(dir))
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	files, err := skillTreeFiles(ctx, root, 0)
	if err != nil {
		return "", errors.Join(err, root.Close())
	}
	digest, err := hashSkillTreeFiles(ctx, root, files)
	return digest, errors.Join(err, root.Close())
}

type skillTreeFile struct {
	path string
	perm fs.FileMode
}

// hashSkillTreeFiles builds the manifest from the enumerated files. Each record takes its mode, size
// and hash from the one file it opened, and a file that vanished since enumeration is a changed
// tree, never an absent one.
func hashSkillTreeFiles(ctx context.Context, root *os.Root, files []skillTreeFile) (string, error) {
	manifest := sha256.New()
	manifest.Write([]byte(skillTreeDigestHeader))
	for _, file := range files {
		perm, size, sum, err := hashRootFile(ctx, root, file.path)
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("%w: %s disappeared while it was read", errSkillTreeChanged, file.path)
		}
		if err != nil {
			return "", err
		}
		manifest.Write([]byte(skillManifestLine(file.path, perm, size, sum)))
	}
	return "sha256:" + hex.EncodeToString(manifest.Sum(nil)), nil
}

// skillManifestLine is one file's v1 manifest record.
func skillManifestLine(path string, perm fs.FileMode, size int64, sum string) string {
	return strconv.Itoa(len(path)) + ":" + path + "\n" + strconv.FormatUint(uint64(perm), 8) + "\n" +
		strconv.FormatInt(size, 10) + "\n" + sum + "\n"
}

// skillTreeFiles lists the regular files of root sorted by path bytes, refusing any other entry type.
func skillTreeFiles(ctx context.Context, root *os.Root, limit int) ([]skillTreeFile, error) {
	var files []skillTreeFile
	err := fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("%w: %s", ErrSkillTreeUnsupported, path)
		}
		if limit > 0 && len(files) == limit {
			return fmt.Errorf("%w: more than %d files", ErrSkillReviewBaseUnsupported, limit)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&skillUnsupportedMode != 0 {
			return fmt.Errorf("%w: %s has special permission bits", ErrSkillTreeUnsupported, path)
		}
		files = append(files, skillTreeFile{path: path, perm: info.Mode().Perm()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(files, func(a, b skillTreeFile) int { return strings.Compare(a.path, b.path) })
	return files, nil
}

func hashRootFile(ctx context.Context, root *os.Root, path string) (fs.FileMode, int64, string, error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, "", err
	}
	file, err := root.Open(path)
	if err != nil {
		return 0, 0, "", err
	}
	info, err := file.Stat()
	if err == nil && (!info.Mode().IsRegular() || info.Mode()&skillUnsupportedMode != 0) {
		err = fmt.Errorf("%w: %s", ErrSkillTreeUnsupported, path)
	}
	if err != nil {
		return 0, 0, "", errors.Join(err, file.Close())
	}
	sum := sha256.New()
	size, err := io.Copy(sum, skillContextReader{ctx: ctx, source: file})
	if err := errors.Join(err, file.Close()); err != nil {
		return 0, 0, "", err
	}
	return info.Mode().Perm(), size, hex.EncodeToString(sum.Sum(nil)), nil
}

// skillTreeState returns the tree digest of dir, or "" when dir itself does not exist. Only that
// first check classifies a skill as absent; a tree that changes while it is read is an error.
func skillTreeState(dir string) (string, error) {
	return skillTreeStateContext(context.Background(), dir)
}

func skillTreeStateContext(ctx context.Context, dir string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return digestSkillTree(ctx, dir)
}

// skillTreeUnexplained stands for a tree recovery cannot digest (a symlink or other unsupported
// entry was added). It never equals a digest or absence, so recovery retains such a tree as a
// conflict instead of failing on it.
const skillTreeUnexplained = "unexplained"

// recoveryTreeState is skillTreeState for recovery classification, mapping an unsupported tree to
// skillTreeUnexplained; every other inspection failure stays an error.
func recoveryTreeState(dir string) (string, error) {
	digest, err := skillTreeState(dir)
	if errors.Is(err, ErrSkillTreeUnsupported) {
		return skillTreeUnexplained, nil
	}
	return digest, err
}

// copySkillTree copies src into the new directory dst byte- and permission-exact and syncs it.
// Symlinks and non-regular entries are refused. A partial dst is left for the caller to remove.
func copySkillTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrSkillTreeUnsupported, filepath.Base(src))
	}
	root, err := os.OpenRoot(src)
	if err != nil {
		return err
	}
	files, err := skillTreeFiles(context.Background(), root, 0)
	if err == nil {
		err = os.Mkdir(dst, 0o700)
	}
	if err == nil {
		for _, file := range files {
			if err = copyRootFile(root, file, dst); err != nil {
				break
			}
		}
	}
	if err == nil {
		err = syncSkillTree(dst)
	}
	return errors.Join(err, root.Close())
}

func copyRootFile(root *os.Root, file skillTreeFile, dst string) error {
	target := filepath.Join(dst, filepath.FromSlash(file.path))
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	source, err := root.Open(file.path)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(source)
	if err := errors.Join(err, source.Close()); err != nil {
		return err
	}
	return writeSkillFile(target, data, file.perm)
}

// writeSkillFile creates path with exactly perm (independent of the umask) and syncs it.
func writeSkillFile(path string, data []byte, perm fs.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if err == nil {
		err = file.Chmod(perm)
	}
	if err == nil {
		err = file.Sync()
	}
	return errors.Join(err, file.Close())
}

// syncSkillTree syncs every directory of dir so a following rename publishes durable contents.
func syncSkillTree(dir string) error {
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return err
		}
		handle, err := fileutil.OpenDirectoryForMutation(path)
		if err != nil {
			return err
		}
		return errors.Join(handle.Sync(), handle.Close())
	})
}
