package workspace

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/frontmatter"
	yaml "gopkg.in/yaml.v3"
)

var errInvalidWorkspaceSkillDefinition = errors.New("workspace: invalid skill definition")

func loadWorkspaceSkillName(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("workspace: read skill identity %q: %w", path, err)
	}

	parts, err := frontmatter.Split(content)
	if err != nil {
		return "", fmt.Errorf(
			"workspace: split skill identity %q: %w: %w",
			path,
			errInvalidWorkspaceSkillDefinition,
			err,
		)
	}

	metadata := struct {
		Name string `yaml:"name"`
	}{}
	if err := yaml.Unmarshal(parts.Metadata, &metadata); err != nil {
		return "", fmt.Errorf(
			"workspace: decode skill identity %q: %w: %w",
			path,
			errInvalidWorkspaceSkillDefinition,
			err,
		)
	}

	name := strings.TrimSpace(metadata.Name)
	if name == "" {
		return "", fmt.Errorf(
			"workspace: decode skill identity %q: %w: name is required",
			path,
			errInvalidWorkspaceSkillDefinition,
		)
	}
	return name, nil
}
