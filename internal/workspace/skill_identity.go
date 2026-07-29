package workspace

import (
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/frontmatter"
	yaml "gopkg.in/yaml.v3"
)

func loadWorkspaceSkillName(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("workspace: read skill identity %q: %w", path, err)
	}

	parts, err := frontmatter.Split(content)
	if err != nil {
		return "", fmt.Errorf("workspace: split skill identity %q: %w", path, err)
	}

	metadata := struct {
		Name string `yaml:"name"`
	}{}
	if err := yaml.Unmarshal(parts.Metadata, &metadata); err != nil {
		return "", fmt.Errorf("workspace: decode skill identity %q: %w", path, err)
	}

	name := strings.TrimSpace(metadata.Name)
	if name == "" {
		return "", fmt.Errorf("workspace: decode skill identity %q: name is required", path)
	}
	return name, nil
}
