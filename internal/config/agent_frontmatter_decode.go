package config

import (
	"errors"
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/goccy/go-yaml"
)

// decodeAgentFrontmatter is the single AGENT.md frontmatter decoder, shared by parse and edit.
func decodeAgentFrontmatter(data []byte, parsed *parsedAgentDef) error {
	if hasEmbeddedTabFrontmatterKey(data) {
		return ErrInvalidAgentFrontmatterKey
	}
	data, err := ignoreRetiredAgentHookMatchers(data)
	if err != nil {
		return err
	}

	yamlErr := yaml.UnmarshalWithOptions(data, parsed, yaml.Strict())
	if yamlErr == nil {
		return nil
	}

	var parsedTOML parsedAgentDef
	meta, tomlErr := toml.Decode(string(data), &parsedTOML)
	if tomlErr != nil {
		return fmt.Errorf(
			"decode agent frontmatter: %w",
			errors.Join(
				fmt.Errorf("yaml: %w", yamlErr),
				fmt.Errorf("toml: %w", tomlErr),
			),
		)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return fmt.Errorf("decode agent frontmatter: unknown field %q", undecoded[0].String())
	}
	*parsed = parsedTOML
	return nil
}
