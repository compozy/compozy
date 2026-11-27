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
	"strings"
)

var (
	ErrSkillReviewBaseMissing     = errors.New("skill_review_base_missing")
	ErrSkillReviewBaseUnsupported = errors.New("skill_review_base_unsupported")
	ErrSkillReviewBaseStale       = errors.New("skill_review_base_stale")
	ErrSkillProposeNoChange       = errors.New("skill_propose_no_change")
	ErrSkillProposeInvalidRemove  = errors.New("skill_propose_invalid_remove")
	// ErrSkillReviewSnapshotInvalid marks a base snapshot that no longer matches its recorded digest.
	ErrSkillReviewSnapshotInvalid = errors.New("skill_review_snapshot_invalid")
)

const (
	maxSkillBaseFiles = 64
	maxSkillBaseBytes = 1 << 20
	// skillNewFilePerm is the mode of every file a proposal adds; replaced files keep their base mode.
	skillNewFilePerm    fs.FileMode = 0o600
	skillSnapshotSuffix             = ".base"
)

// SkillTreeFile is one regular file of a skill tree held in memory.
type SkillTreeFile struct {
	Path string
	Perm fs.FileMode
	Data []byte
}

// SkillTree is a skill's regular files sorted by path bytes.
type SkillTree []SkillTreeFile

// Digest is SkillTreeDigest of the tree as it would be written to disk.
func (t SkillTree) Digest() string {
	manifest := sha256.New()
	manifest.Write([]byte(skillTreeDigestHeader))
	for _, file := range t {
		sum := sha256.Sum256(file.Data)
		manifest.Write(
			[]byte(skillManifestLine(file.Path, file.Perm, int64(len(file.Data)), hex.EncodeToString(sum[:]))),
		)
	}
	return "sha256:" + hex.EncodeToString(manifest.Sum(nil))
}

// AutomaticContents inventories hooks, MCP servers and scripts of the whole tree. Binary files only
// count by path; a malformed declaration returns an error (callers fail closed).
func (t SkillTree) AutomaticContents() (SkillAutomaticContents, error) {
	draft := SkillDraft{Files: make([]SkillDraftFile, 0, len(t))}
	for _, file := range t {
		draft.Files = append(draft.Files, SkillDraftFile{Path: file.Path, Content: string(file.Data)})
	}
	return draft.AutomaticContents()
}

func (t SkillTree) find(path string) (SkillTreeFile, bool) {
	i, found := slices.BinarySearchFunc(t, path, func(file SkillTreeFile, p string) int {
		return strings.Compare(file.Path, p)
	})
	if !found {
		return SkillTreeFile{}, false
	}
	return t[i], true
}

// draftSkillTree is the tree a `new` review installs.
func draftSkillTree(draft SkillDraft) SkillTree {
	tree := make(SkillTree, 0, len(draft.Files))
	for _, file := range draft.Files {
		tree = append(tree, SkillTreeFile{Path: file.Path, Perm: skillNewFilePerm, Data: []byte(file.Content)})
	}
	slices.SortFunc(tree, func(a, b SkillTreeFile) int { return strings.Compare(a.Path, b.Path) })
	return tree
}

// readSkillBase reads an installed skill (or a snapshot) losslessly. A tree with a symlink or another
// non-regular entry, more than 64 files, or more than 1 MiB returns ErrSkillReviewBaseUnsupported.
func readSkillBase(dir string) (SkillTree, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	tree, err := readSkillRoot(root)
	return tree, errors.Join(err, root.Close())
}

func readSkillRoot(root *os.Root) (SkillTree, error) {
	files, err := skillTreeFiles(root, maxSkillBaseFiles)
	if errors.Is(err, ErrSkillTreeUnsupported) {
		return nil, fmt.Errorf("%w: %w", ErrSkillReviewBaseUnsupported, err)
	}
	if err != nil {
		return nil, err
	}
	tree := make(SkillTree, 0, len(files))
	total := 0
	for _, file := range files {
		if !validSkillTreePath(file.path) {
			return nil, fmt.Errorf("%w: invalid file path %q", ErrSkillReviewBaseUnsupported, file.path)
		}
		handle, err := root.Open(file.path)
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(handle, maxSkillBaseBytes+1))
		if err := errors.Join(err, handle.Close()); err != nil {
			return nil, err
		}
		total += len(data)
		if total > maxSkillBaseBytes {
			return nil, fmt.Errorf("%w: larger than 1 MiB", ErrSkillReviewBaseUnsupported)
		}
		tree = append(tree, SkillTreeFile{Path: file.path, Perm: file.perm, Data: data})
	}
	return tree, nil
}

// readSkillEditBase reads the installed skill an edit applies to: absent ⇒ base missing.
func readSkillEditBase(dir, name string) (SkillTree, error) {
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: no skill named %s is installed for this instance; use mode new",
			ErrSkillReviewBaseMissing, name)
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s is not a directory", ErrSkillReviewBaseUnsupported, name)
	}
	return readSkillBase(dir)
}

// prepareSkillRemovePaths validates remove_paths: edit only, valid relative file paths, never
// SKILL.md (it is regenerated), never a path the proposal also writes. Returned sorted and distinct.
func prepareSkillRemovePaths(mode SkillProposalMode, paths []string, files []SkillDraftFile) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	if mode != SkillProposalEdit {
		return nil, fmt.Errorf("%w: remove_paths is only allowed with mode edit", ErrSkillProposeInvalidRemove)
	}
	written := make(map[string]bool, len(files)+1)
	written[skillDefinitionFile] = true
	for _, file := range files {
		written[file.Path] = true
	}
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if !validSkillTreePath(path) || written[path] {
			return nil, fmt.Errorf("%w: %q cannot be removed", ErrSkillProposeInvalidRemove, path)
		}
		result = append(result, path)
	}
	slices.Sort(result)
	return slices.Compact(result), nil
}

// applySkillOverlay returns the resulting tree of an edit: draft files add or replace base files (a
// replaced file keeps its base mode), RemovePaths delete base files, and every other base file is kept
// byte- and mode-exact.
func applySkillOverlay(base SkillTree, draft SkillDraft) (SkillTree, error) {
	byPath := make(map[string]SkillTreeFile, len(base)+len(draft.Files))
	folded := make(map[string]string, len(base))
	for _, file := range base {
		byPath[file.Path] = file
		folded[strings.ToLower(file.Path)] = file.Path
	}
	for _, path := range draft.RemovePaths {
		if _, ok := byPath[path]; !ok {
			return nil, fmt.Errorf("%w: %s is not a file of the installed skill", ErrSkillProposeInvalidRemove, path)
		}
		delete(byPath, path)
	}
	for _, file := range draft.Files {
		if existing, ok := folded[strings.ToLower(file.Path)]; ok && existing != file.Path {
			return nil, fmt.Errorf("%w: skill proposal file %s differs only in case from installed %s",
				ErrSkillReviewInvalid, file.Path, existing)
		}
		perm := skillNewFilePerm
		if previous, ok := byPath[file.Path]; ok {
			perm = previous.Perm
		}
		byPath[file.Path] = SkillTreeFile{Path: file.Path, Perm: perm, Data: []byte(file.Content)}
	}
	result := make(SkillTree, 0, len(byPath))
	for _, file := range byPath {
		result = append(result, file)
	}
	slices.SortFunc(result, func(a, b SkillTreeFile) int { return strings.Compare(a.Path, b.Path) })
	clear(folded)
	for _, file := range result {
		key := strings.ToLower(file.Path)
		if existing, ok := folded[key]; ok {
			return nil, fmt.Errorf("%w: skill tree files %s and %s differ only in case",
				ErrSkillReviewInvalid, existing, file.Path)
		}
		folded[key] = file.Path
	}
	for _, file := range result {
		parent := filepath.ToSlash(filepath.Dir(file.Path))
		for ; parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			if existing, ok := folded[strings.ToLower(parent)]; ok {
				return nil, fmt.Errorf("%w: skill proposal file %s conflicts with the file %s", ErrSkillReviewInvalid,
					file.Path, existing)
			}
		}
	}
	return result, nil
}

// skillEdit is the captured base and the tree approval will install.
type skillEdit struct {
	base, result               SkillTree
	baseDigest, proposedDigest string
}

// prepareSkillEdit captures the live base of an edit under the skill locks and computes the resulting
// tree; a result identical to the base is refused.
func prepareSkillEdit(dir string, draft SkillDraft) (skillEdit, error) {
	base, err := readSkillEditBase(dir, draft.Name)
	if err != nil {
		return skillEdit{}, err
	}
	result, err := applySkillOverlay(base, draft)
	if err != nil {
		return skillEdit{}, err
	}
	edit := skillEdit{base: base, result: result, baseDigest: base.Digest(), proposedDigest: result.Digest()}
	if edit.baseDigest == edit.proposedDigest {
		return skillEdit{}, fmt.Errorf("%w: the proposal equals the installed %s", ErrSkillProposeNoChange, draft.Name)
	}
	return edit, nil
}

// skillSnapshotPath is the base snapshot of a review, next to its draft (outside every scan root).
func skillSnapshotPath(draftPath, reviewID string) string {
	return filepath.Join(filepath.Dir(draftPath), reviewID+skillSnapshotSuffix)
}

// writeSkillSnapshot writes the base byte- and mode-exact into the new directory dir and verifies it.
// Snapshots are never redacted.
func writeSkillSnapshot(dir string, base SkillTree, digest string) error {
	if err := os.Mkdir(dir, 0o700); err != nil {
		return err
	}
	err := writeSkillTreeFiles(dir, base)
	if err == nil {
		err = syncSkillTree(dir)
	}
	if err == nil {
		var written string
		written, err = SkillTreeDigest(dir)
		if err == nil && written != digest {
			err = errors.New("skill base snapshot does not match the captured base")
		}
	}
	if err != nil {
		return errors.Join(err, os.RemoveAll(dir))
	}
	return nil
}

// writeSkillTreeFiles writes every file of tree into the existing directory dir with its exact mode.
func writeSkillTreeFiles(dir string, tree SkillTree) error {
	for _, file := range tree {
		if !validSkillTreePath(file.Path) {
			return ErrSkillReviewInvalid
		}
		path := filepath.Join(dir, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := writeSkillFile(path, file.Data, file.Perm); err != nil {
			return err
		}
	}
	return nil
}

func validSkillTreePath(path string) bool {
	return fs.ValidPath(path) && path != "." && !strings.ContainsAny(path, "\\\x00:")
}

// readSkillSnapshot reads a review's base snapshot and re-verifies it against the base digest.
func readSkillSnapshot(review SkillReview) (SkillTree, error) {
	if review.SnapshotPath == "" {
		return nil, ErrSkillReviewSnapshotInvalid
	}
	base, err := readSkillBase(review.SnapshotPath)
	if err != nil || base.Digest() != review.BaseDigest {
		return nil, errors.Join(ErrSkillReviewSnapshotInvalid, err)
	}
	return base, nil
}
