package setup

import (
	"bufio"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
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
	file, err := bundle.Open(skillPath)
	if err != nil {
		return Skill{}, fmt.Errorf("read bundled skill %q: %w", dir, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return Skill{}, fmt.Errorf("read bundled skill %q: empty SKILL.md", dir)
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return Skill{}, fmt.Errorf("read bundled skill %q: missing YAML frontmatter", dir)
	}

	var (
		name        string
		description string
	)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = trimYAMLString(value)
		switch key {
		case "name":
			name = value
		case "description":
			description = value
		}
	}
	if err := scanner.Err(); err != nil {
		return Skill{}, fmt.Errorf("read bundled skill %q: %w", dir, err)
	}
	if name == "" || description == "" {
		return Skill{}, fmt.Errorf("read bundled skill %q: missing name or description", dir)
	}

	return Skill{
		Name:        name,
		Description: description,
		Directory:   dir,
	}, nil
}

func trimYAMLString(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 2 {
		if (trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') ||
			(trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'') {
			return trimmed[1 : len(trimmed)-1]
		}
	}
	return trimmed
}
