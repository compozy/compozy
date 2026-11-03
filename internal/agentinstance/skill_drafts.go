package agentinstance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/skills"
	yaml "gopkg.in/yaml.v3"
)

var ErrSkillProposeUnverified = errors.New("skill_propose_unverified")

var proposedSkillName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

const maxSkillDraftBytes = 1 << 20

type SkillDraftFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type SkillDraft struct {
	Name  string           `json:"name"`
	Files []SkillDraftFile `json:"files"`
}

func PrepareSkillDraft(name, description, body string, files []SkillDraftFile) (SkillDraft, error) {
	if !proposedSkillName.MatchString(name) || strings.TrimSpace(description) == "" || strings.TrimSpace(body) == "" ||
		len(files) > 64 {
		return SkillDraft{}, errors.New(
			"skill proposal requires a valid name, description, body and at most 64 supporting files",
		)
	}
	frontmatter, err := yaml.Marshal(struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}{Name: name, Description: description})
	if err != nil {
		return SkillDraft{}, err
	}
	result := SkillDraft{
		Name:  name,
		Files: []SkillDraftFile{{Path: "SKILL.md", Content: "---\n" + string(frontmatter) + "---\n\n" + body}},
	}
	result.Files = append(result.Files, files...)
	slices.SortFunc(result.Files, func(a, b SkillDraftFile) int { return strings.Compare(a.Path, b.Path) })
	seen := make(map[string]bool, len(result.Files))
	total := 0
	for i, file := range result.Files {
		key := strings.ToLower(file.Path)
		if !fs.ValidPath(file.Path) || file.Path == "." || strings.ContainsAny(file.Path, "\\\x00:") || seen[key] ||
			!utf8.ValidString(file.Content) {
			return SkillDraft{}, errors.New("skill proposal contains an invalid or duplicate file")
		}
		seen[key] = true
		for _, warning := range skills.VerifyContent(file.Content) {
			if warning.Severity == skills.SeverityCritical {
				return SkillDraft{}, ErrSkillProposeUnverified
			}
		}
		result.Files[i].Content = redact.String(file.Content)
		total += len(file.Path) + len(result.Files[i].Content)
		if total > maxSkillDraftBytes {
			return SkillDraft{}, errors.New("skill proposal exceeds 1 MiB")
		}
	}
	for path := range seen {
		for parent := filepath.ToSlash(filepath.Dir(path)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			if seen[parent] {
				return SkillDraft{}, errors.New("skill proposal file conflicts with a directory")
			}
		}
	}
	return result, nil
}

func WriteSkillDraft(root string, scanRoots []string, draft SkillDraft) (path, digest string, err error) {
	if root == "" || len(scanRoots) == 0 {
		return "", "", errors.New("skill drafts require a root and complete scan roots")
	}
	canonical, err := fileutil.CanonicalPathWithExistingPrefix(root)
	if err != nil {
		return "", "", err
	}
	for _, scanRoot := range scanRoots {
		if scanRoot == "" {
			return "", "", errors.New("skill scan root is empty")
		}
		canonicalScanRoot, err := fileutil.CanonicalPathWithExistingPrefix(scanRoot)
		if err != nil {
			return "", "", err
		}
		within, err := fileutil.PathWithinRoot(canonicalScanRoot, canonical)
		if err != nil {
			return "", "", err
		}
		if within {
			return "", "", errors.New("skill drafts must be outside every scan root")
		}
	}
	data, err := json.Marshal(draft)
	if err != nil {
		return "", "", err
	}
	if len(data) > maxSkillDraftBytes {
		return "", "", errors.New("encoded skill proposal exceeds 1 MiB")
	}
	if err := os.MkdirAll(canonical, 0o700); err != nil {
		return "", "", err
	}
	file, err := os.CreateTemp(canonical, "skill-review-*.json")
	if err != nil {
		return "", "", err
	}
	path = file.Name()
	_, writeErr := file.Write(data)
	err = errors.Join(writeErr, file.Close())
	if err != nil {
		return "", "", errors.Join(err, os.Remove(path))
	}
	return path, skillDraftDigest(data), nil
}

func ReadSkillDraft(path, digest string) (SkillDraft, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return SkillDraft{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxSkillDraftBytes {
		return SkillDraft{}, ErrSkillReviewStale
	}
	file, err := os.Open(path)
	if err != nil {
		return SkillDraft{}, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxSkillDraftBytes+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return SkillDraft{}, err
	}
	if len(data) > maxSkillDraftBytes {
		return SkillDraft{}, ErrSkillReviewStale
	}
	if skillDraftDigest(data) != digest {
		return SkillDraft{}, ErrSkillReviewStale
	}
	var draft SkillDraft
	if err := json.Unmarshal(data, &draft); err != nil {
		return SkillDraft{}, fmt.Errorf("read skill draft: %w", err)
	}
	return draft, nil
}

func skillDraftDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}
