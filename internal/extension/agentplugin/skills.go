package agentplugin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	discoverSkillsDirectory(root, filepath.Join(root, "skills"), pkg)
}

func discoverSkillsDirectory(root, skillsDir string, pkg *Package) {
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
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			pkg.Diagnostics = append(
				pkg.Diagnostics,
				Diagnostic{Scope: "skill:" + entry.Name(), Message: "skill entry must be an in-root directory"},
			)
			continue
		}
		discoverSkill(root, filepath.Join(skillsDir, entry.Name()), pkg)
	}
}

func discoverSkill(root, dir string, pkg *Package) {
	name := filepath.Base(dir)
	scope := "skill:" + name
	skillFile := filepath.Join(dir, "SKILL.md")
	info, err := os.Lstat(skillFile)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{Scope: scope, Message: "cannot inspect SKILL.md"})
		return
	}
	if !info.Mode().IsRegular() {
		pkg.Diagnostics = append(
			pkg.Diagnostics,
			Diagnostic{Scope: scope, Message: "SKILL.md must be a regular in-root file"},
		)
		return
	}
	if _, err := resolveContained(skillFile, root); err != nil {
		pkg.Diagnostics = append(
			pkg.Diagnostics,
			Diagnostic{Scope: scope, Message: "SKILL.md must remain inside the package root"},
		)
		return
	}
	metadata, err := readSkillMetadata(skillFile)
	if err == nil {
		err = validateSkillMetadata(name, metadata)
	}
	if err != nil {
		pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{Scope: scope, Message: err.Error()})
		return
	}
	pkg.Skills = append(pkg.Skills, SkillRef{Name: metadata.Name, Dir: dir, SkillFile: skillFile})
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
