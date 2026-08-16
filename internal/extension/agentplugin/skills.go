package agentplugin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/frontmatter"
)

type skillMetadata struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license,omitempty"`
	Compatibility string            `yaml:"compatibility,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
	AllowedTools  string            `yaml:"allowed-tools,omitempty"`
}

func discoverSkills(root string, pkg *Package) {
	skillsDir := filepath.Join(root, "skills")
	info, err := os.Lstat(skillsDir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			pkg.Diagnostics = append(
				pkg.Diagnostics,
				Diagnostic{Scope: scopeSkills, Message: "cannot inspect skills directory"},
			)
		}
		return
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		pkg.Diagnostics = append(
			pkg.Diagnostics,
			Diagnostic{Scope: scopeSkills, Message: "skills must be an in-root directory"},
		)
		return
	}
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		pkg.Diagnostics = append(
			pkg.Diagnostics,
			Diagnostic{Scope: scopeSkills, Message: "cannot read skills directory"},
		)
		return
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name() < entries[right].Name() })
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		dir := filepath.Join(skillsDir, entry.Name())
		skillFile := filepath.Join(dir, "SKILL.md")
		fileInfo, err := os.Lstat(skillFile)
		if err != nil || !fileInfo.Mode().IsRegular() {
			continue
		}
		if _, err := resolveContained(skillFile, root); err != nil {
			pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{
				Scope: "skill:" + entry.Name(), Message: "SKILL.md must remain inside the package root",
			})
			continue
		}
		metadata, err := readSkillMetadata(skillFile)
		if err != nil {
			pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{
				Scope: "skill:" + entry.Name(), Message: err.Error(),
			})
			continue
		}
		if err := validateSkillMetadata(entry.Name(), metadata); err != nil {
			pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{
				Scope: "skill:" + entry.Name(), Message: err.Error(),
			})
			continue
		}
		pkg.Skills = append(pkg.Skills, SkillRef{Name: metadata.Name, Dir: dir, SkillFile: skillFile})
	}
}

func readSkillMetadata(path string) (skillMetadata, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return skillMetadata{}, fmt.Errorf("cannot read SKILL.md: %w", err)
	}
	parts, err := frontmatter.Split(content)
	if err != nil {
		return skillMetadata{}, fmt.Errorf("invalid SKILL.md frontmatter: %w", err)
	}
	var metadata skillMetadata
	if err := frontmatter.UnmarshalMetadata(parts.Metadata, &metadata); err != nil {
		return skillMetadata{}, fmt.Errorf("invalid SKILL.md frontmatter: %w", err)
	}
	return metadata, nil
}

func validateSkillMetadata(dirName string, metadata skillMetadata) error {
	if metadata.Name == "" {
		return errors.New("name is required")
	}
	if metadata.Name != dirName {
		return fmt.Errorf("name must match the directory %q", dirName)
	}
	if err := validateSkillName(metadata.Name); err != nil {
		return err
	}
	if strings.TrimSpace(metadata.Description) == "" {
		return errors.New("description is required and must be non-empty")
	}
	if utf8.RuneCountInString(metadata.Description) > 1024 {
		return errors.New("description must not exceed 1024 characters")
	}
	if utf8.RuneCountInString(metadata.Compatibility) > 500 {
		return errors.New("compatibility must not exceed 500 characters")
	}
	return nil
}

func validateSkillName(name string) error {
	if name == "" || len(name) > 64 {
		return errors.New("skill name length must be between 1 and 64 bytes")
	}
	if strings.Contains(name, "--") {
		return errors.New("skill name must not contain consecutive hyphens")
	}
	for _, character := range []byte(name) {
		if isASCIILower(character) || isASCIIDigit(character) || character == '-' {
			continue
		}
		return errors.New("skill name may contain only lowercase ASCII letters, digits, and hyphens")
	}
	if !isASCIIAlphanumeric(name[0]) || !isASCIIAlphanumeric(name[len(name)-1]) {
		return errors.New("skill name must start and end with an ASCII letter or digit")
	}
	return nil
}
