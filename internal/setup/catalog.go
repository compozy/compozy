package setup

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/core/frontmatter"
)

// ListSkills enumerates bundled public skills from the provided bundle.
func ListSkills(bundle fs.FS) ([]Skill, error) {
	if bundle == nil {
		return nil, fmt.Errorf("list bundled skills: bundle is nil")
	}

	entries, err := fs.ReadDir(bundle, ".")
	if err != nil {
		return nil, fmt.Errorf("list bundled skills: %w", err)
	}

	skills := make([]Skill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skill, err := parseSkill(bundle, entry.Name())
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}

	slices.SortFunc(skills, func(left, right Skill) int {
		return strings.Compare(left.Name, right.Name)
	})
	return skills, nil
}

func parseSkill(bundle fs.FS, dir string) (Skill, error) {
	skillPath := path.Join(dir, "SKILL.md")
	content, err := fs.ReadFile(bundle, skillPath)
	if err != nil {
		return Skill{}, fmt.Errorf("read bundled skill %q: %w", dir, err)
	}

	var metadata struct {
		Name         string `yaml:"name"`
		Description  string `yaml:"description"`
		ArgumentHint any    `yaml:"argument-hint,omitempty"`
	}
	if _, err := frontmatter.Parse(string(content), &metadata); err != nil {
		return Skill{}, fmt.Errorf("read bundled skill %q: %w", dir, err)
	}
	if metadata.Name == "" || metadata.Description == "" {
		return Skill{}, fmt.Errorf("read bundled skill %q: missing name or description", dir)
	}

	return Skill{
		Name:        metadata.Name,
		Description: metadata.Description,
		Directory:   dir,
	}, nil
}
