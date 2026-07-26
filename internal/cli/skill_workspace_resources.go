package cli

import (
	"errors"
	"fmt"
	"io/fs"

	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/skills"
)

func listSkillResources(skill *skills.Skill, bundledFS fs.FS) ([]string, error) {
	if skill == nil {
		return nil, errors.New("skill is required")
	}

	resources := make([]string, 0)
	switch skill.Source {
	case skills.SourceBundled:
		if bundledFS == nil {
			return nil, errors.New("bundled skills filesystem is required")
		}

		root := strings.TrimSpace(skill.Dir)
		if root == "" {
			return []string{}, nil
		}

		err := fs.WalkDir(bundledFS, root, func(resourcePath string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}

			relative := strings.TrimPrefix(resourcePath, root+"/")
			if resourcePath == root {
				relative = skillMarkdownFileName
			}
			if relative == skillMarkdownFileName {
				return nil
			}

			resources = append(resources, relative)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("cli: list bundled skill resources for %q: %w", skill.Meta.Name, err)
		}
	default:
		root := strings.TrimSpace(skill.Dir)
		if root == "" {
			return []string{}, nil
		}

		err := filepath.WalkDir(root, func(resourcePath string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}

			relative, err := filepath.Rel(root, resourcePath)
			if err != nil {
				return err
			}
			if filepath.Clean(relative) == skillMarkdownFileName {
				return nil
			}

			resources = append(resources, filepath.ToSlash(relative))
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("cli: list skill resources for %q: %w", skill.Meta.Name, err)
		}
	}

	sort.Strings(resources)
	return resources, nil
}

func readSkillResource(skill *skills.Skill, bundledFS fs.FS, relativePath string) (string, error) {
	if skill == nil {
		return "", errors.New("skill is required")
	}

	switch skill.Source {
	case skills.SourceBundled:
		if bundledFS == nil {
			return "", errors.New("bundled skills filesystem is required")
		}

		cleanPath, err := cleanBundledSkillRelativePath(relativePath)
		if err != nil {
			return "", err
		}
		root := strings.TrimSpace(skill.Dir)
		if root == "" {
			return "", errors.New("skill directory is required")
		}

		content, err := fs.ReadFile(bundledFS, path.Join(root, cleanPath))
		if err != nil {
			return "", fmt.Errorf("cli: read bundled skill file %q: %w", cleanPath, err)
		}
		return string(content), nil
	default:
		cleanPath, err := cleanFilesystemSkillRelativePath(relativePath)
		if err != nil {
			return "", err
		}

		root := strings.TrimSpace(skill.Dir)
		if root == "" {
			return "", errors.New("skill directory is required")
		}

		targetPath := filepath.Join(root, cleanPath)
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return "", fmt.Errorf("cli: resolve skill directory %q: %w", root, err)
		}
		resolvedRoot, err := filepath.EvalSymlinks(absRoot)
		if err != nil {
			return "", fmt.Errorf("cli: resolve skill directory %q: %w", absRoot, err)
		}
		absTarget, err := filepath.Abs(targetPath)
		if err != nil {
			return "", fmt.Errorf("cli: resolve skill file %q: %w", targetPath, err)
		}
		resolvedTarget, err := filepath.EvalSymlinks(absTarget)
		if err != nil {
			return "", fmt.Errorf("cli: resolve skill file %q: %w", absTarget, err)
		}

		relativeToRoot, err := filepath.Rel(resolvedRoot, resolvedTarget)
		if err != nil {
			return "", fmt.Errorf("cli: resolve skill file %q within %q: %w", resolvedTarget, resolvedRoot, err)
		}
		if relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
			return "", errors.New("skill file path must stay within the skill directory")
		}

		content, err := os.ReadFile(resolvedTarget)
		if err != nil {
			return "", fmt.Errorf("cli: read skill file %q: %w", cleanPath, err)
		}
		return string(content), nil
	}
}

func cleanBundledSkillRelativePath(relativePath string) (string, error) {
	cleaned := path.Clean(strings.TrimSpace(strings.ReplaceAll(relativePath, "\\", "/")))
	switch {
	case cleaned == ".", cleaned == "":
		return "", errors.New("skill file path is required")
	case strings.HasPrefix(cleaned, "/"):
		return "", errors.New("skill file path must be relative")
	case cleaned == "..", strings.HasPrefix(cleaned, "../"):
		return "", errors.New("skill file path must stay within the skill directory")
	default:
		return cleaned, nil
	}
}

func cleanFilesystemSkillRelativePath(relativePath string) (string, error) {
	cleaned := filepath.Clean(strings.TrimSpace(relativePath))
	switch {
	case cleaned == ".", cleaned == "":
		return "", errors.New("skill file path is required")
	case filepath.IsAbs(cleaned):
		return "", errors.New("skill file path must be relative")
	case cleaned == "..", strings.HasPrefix(cleaned, ".."+string(filepath.Separator)):
		return "", errors.New("skill file path must stay within the skill directory")
	default:
		return cleaned, nil
	}
}

func renderSkillXML(skill *skills.Skill, content string, resources []string) (string, error) {
	if skill == nil {
		return "", errors.New("skill is required")
	}

	var builder strings.Builder
	builder.WriteString(`<skill_content name="`)
	builder.WriteString(skillXMLAttributeReplacer.Replace(skill.Meta.Name))
	builder.WriteString(`">`)
	builder.WriteString("\n")
	builder.WriteString(skillXMLTextReplacer.Replace(content))
	if !strings.HasSuffix(content, "\n") {
		builder.WriteString("\n")
	}
	builder.WriteString("\n<skill_resources>\n")
	for _, resource := range resources {
		builder.WriteString("  <file>")
		builder.WriteString(skillXMLTextReplacer.Replace(resource))
		builder.WriteString("</file>\n")
	}
	builder.WriteString("</skill_resources>\n")
	builder.WriteString("</skill_content>")
	return builder.String(), nil
}
