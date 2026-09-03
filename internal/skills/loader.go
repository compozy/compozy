package skills

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/skillscan"
	yaml "gopkg.in/yaml.v3"
)

const (
	loaderVersionKey = "version"
	skillAgentKind   = "agent"
)

const skillFileName = skillscan.SkillFileName

var (
	// ErrInvalidDefinition reports malformed or incomplete SKILL.md metadata.
	ErrInvalidDefinition = errors.New("skills: invalid definition")
	errSkillNameRequired = errors.New("skills: skill name is required")
)

var allowedFrontmatterFields = map[string]struct{}{
	"name":                     {},
	"description":              {},
	loaderVersionKey:           {},
	"metadata":                 {},
	"license":                  {},
	"compatibility":            {},
	"allowed-tools":            {},
	"when_to_use":              {},
	"argument-hint":            {},
	"arguments":                {},
	"disable-model-invocation": {},
	"user-invocable":           {},
	"disallowed-tools":         {},
	"model":                    {},
	"effort":                   {},
	"context":                  {},
	skillAgentKind:             {},
	"background":               {},
	"hooks":                    {},
	"paths":                    {},
	"shell":                    {},
	"aliases":                  {},
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
		return nil, "", fmt.Errorf("skills: parse %q: %w: %w", filePath, ErrInvalidDefinition, err)
	}
	if meta.Name == "" {
		return nil, "", fmt.Errorf(
			"skills: parse %q: %w: %w",
			filePath,
			ErrInvalidDefinition,
			errSkillNameRequired,
		)
	}

	skill := &Skill{
		Meta:       meta,
		Source:     source,
		Dir:        dir,
		FilePath:   filePath,
		Enabled:    true,
		Activation: SkillActivation{Active: true},
	}
	if err := parseCompozyMetadata(skill); err != nil {
		return nil, "", fmt.Errorf("skills: parse %q metadata.compozy: %w", filePath, err)
	}
	refreshSkillHookDecls(skill)
	if skill.Meta.Description == "" {
		slog.Warn("skills: parsed skill without description", "path", filePath, "name", skill.Meta.Name)
	}

	return skill, body, nil
}

// scanDirectory returns every SKILL.md file discovered under dir.
func scanDirectory(dir string) ([]string, error) {
	result, err := skillscan.ScanDirectory(dir)
	if err != nil {
		return nil, fmt.Errorf("skills: scan directory %q: %w", dir, err)
	}
	return result.Paths, nil
}

func scanDirectoryWithSnapshots(dir string) ([]string, map[string]filesnap.Snapshot, error) {
	result, err := skillscan.ScanDirectory(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("skills: scan directory %q: %w", dir, err)
	}
	return result.Paths, result.Snapshots, nil
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
