package agentinstance

import (
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

// SkillTreeDigest returns "sha256:<hex>" over the v1 manifest (ADR-002):
//
//	"compozy-skill-tree-v1\n" then, for each regular file sorted by path bytes,
//	"<len(path)>:<path>\n<perm octal>\n<size>\n<sha256(content) hex>\n".
//
// Paths are slash-separated and relative to dir. Directories contribute only through their files.
// Symlinks and non-regular entries return ErrSkillTreeUnsupported.
func SkillTreeDigest(dir string) (string, error) {
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
	digest, err := skillTreeDigest(root)
	return digest, errors.Join(err, root.Close())
}

type skillTreeFile struct {
	path string
	perm fs.FileMode
}

func skillTreeDigest(root *os.Root) (string, error) {
	files, err := skillTreeFiles(root)
	if err != nil {
		return "", err
	}
	return hashSkillTreeFiles(root, files)
}

// hashSkillTreeFiles builds the manifest from the enumerated files. Each record takes its mode, size
// and hash from the one file it opened, and a file that vanished since enumeration is a changed
// tree, never an absent one.
func hashSkillTreeFiles(root *os.Root, files []skillTreeFile) (string, error) {
	manifest := sha256.New()
	manifest.Write([]byte(skillTreeDigestHeader))
	for _, file := range files {
		perm, size, sum, err := hashRootFile(root, file.path)
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
func skillTreeFiles(root *os.Root) ([]skillTreeFile, error) {
	var files []skillTreeFile
	err := fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("%w: %s", ErrSkillTreeUnsupported, path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
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

func hashRootFile(root *os.Root, path string) (fs.FileMode, int64, string, error) {
	file, err := root.Open(path)
	if err != nil {
		return 0, 0, "", err
	}
	info, err := file.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = fmt.Errorf("%w: %s", ErrSkillTreeUnsupported, path)
	}
	if err != nil {
		return 0, 0, "", errors.Join(err, file.Close())
	}
	sum := sha256.New()
	size, err := io.Copy(sum, file)
	if err := errors.Join(err, file.Close()); err != nil {
		return 0, 0, "", err
	}
	return info.Mode().Perm(), size, hex.EncodeToString(sum.Sum(nil)), nil
}

// skillTreeState returns the tree digest of dir, or "" when dir itself does not exist. Only that
// first check classifies a skill as absent; a tree that changes while it is read is an error.
func skillTreeState(dir string) (string, error) {
	if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return SkillTreeDigest(dir)
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
	files, err := skillTreeFiles(root)
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
