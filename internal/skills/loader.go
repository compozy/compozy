package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/filesnap"
	yaml "gopkg.in/yaml.v3"
)

const (
	loaderVersionKey = "version"
)

const (
	skillFileName     = "SKILL.md"
	maxScanDepth      = 4
	maxScanCandidates = 300
)

var (
	errSkillNameRequired = errors.New("skills: skill name is required")
	errScanLimitReached  = errors.New("skills: scan candidate limit reached")
)

var allowedFrontmatterFields = map[string]struct{}{
	"name":           {},
	"description":    {},
	loaderVersionKey: {},
	"metadata":       {},
}

// ParseSkillFile reads and parses a SKILL.md file from disk.
//
// The loader fills parsed metadata plus canonical file locations. The skill
// body is intentionally not retained on the returned Skill; callers must use
// ReadSkillContent when they need the full instructions.
func ParseSkillFile(path string) (*Skill, error) {
	skill, _, err := parseSkillFileDocument(path)
	return skill, err
}

// ParseSkillFileWithSource reads and parses a skill file from disk while
// preserving the caller-selected source tier for downstream precedence and hook
// metadata handling.
func ParseSkillFileWithSource(path string, source SkillSource) (*Skill, error) {
	absPath, content, err := readSkillFile(path)
	if err != nil {
		return nil, err
	}

	skill, _, err := parseSkillDocument(absPath, filepath.Dir(absPath), content, source)
	if err != nil {
		return nil, err
	}
	if err := mergeSkillMCPSidecarFile(filepath.Dir(absPath), skill); err != nil {
		return nil, fmt.Errorf("skills: parse %q MCP JSON: %w", absPath, err)
	}
	return skill, nil
}

// ReadSkillContent reads and returns the markdown body from a SKILL.md file.
func ReadSkillContent(path string) (string, error) {
	_, body, err := parseSkillFileDocument(path)
	if err != nil {
		return "", err
	}
	return body, nil
}

func parseSkillFileDocument(path string) (*Skill, string, error) {
	absPath, content, err := readSkillFile(path)
	if err != nil {
		return nil, "", err
	}

	skill, body, err := parseSkillDocument(absPath, filepath.Dir(absPath), content, 0)
	if err != nil {
		return nil, "", err
	}
	if err := mergeSkillMCPSidecarFile(filepath.Dir(absPath), skill); err != nil {
		return nil, "", fmt.Errorf("skills: parse %q MCP JSON: %w", absPath, err)
	}

	return skill, body, nil
}

func readSkillFile(path string) (string, []byte, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", nil, fmt.Errorf("skills: resolve path %q: %w", path, err)
	}
	if err := ensurePathWithinRoot(filepath.Dir(absPath), absPath); err != nil {
		return "", nil, err
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", nil, fmt.Errorf("skills: read %q: %w", absPath, err)
	}

	return absPath, content, nil
}

func parseSkillDocument(filePath string, dir string, content []byte, source SkillSource) (*Skill, string, error) {
	meta, body, err := parseSkillContent(content)
	if err != nil {
		return nil, "", fmt.Errorf("skills: parse %q: %w", filePath, err)
	}
	if meta.Name == "" {
		return nil, "", fmt.Errorf("skills: parse %q: %w", filePath, errSkillNameRequired)
	}

	skill := &Skill{
		Meta:       meta,
		Source:     source,
		Dir:        dir,
		FilePath:   filePath,
		Enabled:    true,
		Activation: SkillActivation{Active: true},
	}
	if err := parseAGHMetadata(skill); err != nil {
		return nil, "", fmt.Errorf("skills: parse %q metadata.agh: %w", filePath, err)
	}
	refreshSkillHookDecls(skill)
	if skill.Meta.Description == "" {
		slog.Warn("skills: parsed skill without description", "path", filePath, "name", skill.Meta.Name)
	}

	return skill, body, nil
}

// scanDirectory returns every SKILL.md file discovered under dir.
func scanDirectory(dir string) ([]string, error) {
	paths, _, err := scanDirectoryWithSnapshots(dir)
	return paths, err
}

func scanDirectoryWithSnapshots(dir string) ([]string, map[string]filesnap.Snapshot, error) {
	root := strings.TrimSpace(dir)
	if root == "" {
		return nil, nil, errors.New("skills: scan directory root is required")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, fmt.Errorf("skills: resolve scan root %q: %w", dir, err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, map[string]filesnap.Snapshot{}, nil
		}
		return nil, nil, fmt.Errorf("skills: stat scan root %q: %w", absRoot, err)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("skills: scan root %q is not a directory", absRoot)
	}

	paths := make([]string, 0, maxScanCandidates)
	snapshots := make(map[string]filesnap.Snapshot, maxScanCandidates)
	walkErr := filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			slog.Warn("skills: skipping unreadable path during scan", "path", path, "error", walkErr)
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		depth, err := scanDepth(absRoot, path, entry.IsDir())
		if err != nil {
			return fmt.Errorf("skills: determine scan depth for %q: %w", path, err)
		}

		if entry.IsDir() {
			if path != absRoot && shouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			if depth > maxScanDepth {
				return filepath.SkipDir
			}
			return nil
		}

		return appendSkillScanCandidate(absRoot, path, entry, depth, &paths, snapshots)
	})
	if walkErr != nil && !errors.Is(walkErr, errScanLimitReached) {
		return nil, nil, walkErr
	}

	slices.Sort(paths)
	return paths, snapshots, nil
}

func appendSkillScanCandidate(
	absRoot string,
	path string,
	entry fs.DirEntry,
	depth int,
	paths *[]string,
	snapshots map[string]filesnap.Snapshot,
) error {
	if depth > maxScanDepth || entry.Name() != skillFileName {
		return nil
	}
	if err := ensurePathWithinRoot(absRoot, path); err != nil {
		slog.Warn("skills: skipping skill file that escapes scan root", "path", path, "error", err)
		return nil
	}

	snapshot, err := filesnap.FromPath(path)
	if err != nil {
		slog.Warn("skills: skipping unreadable skill file during scan", "path", path, "error", err)
		return nil
	}

	*paths = append(*paths, path)
	snapshots[path] = snapshot
	if len(*paths) >= maxScanCandidates {
		slog.Warn("skills: scan candidate limit reached", "root", absRoot, "limit", maxScanCandidates)
		return errScanLimitReached
	}

	return nil
}

func decodeSkillMeta(frontmatter string) (SkillMeta, error) {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(frontmatter), &document); err != nil {
		return SkillMeta{}, err
	}

	warnUnknownFields(&document)

	var meta SkillMeta
	if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
		return SkillMeta{}, err
	}

	meta.Name = strings.TrimSpace(meta.Name)
	meta.Description = strings.TrimSpace(meta.Description)
	meta.Version = strings.TrimSpace(meta.Version)

	return meta, nil
}
