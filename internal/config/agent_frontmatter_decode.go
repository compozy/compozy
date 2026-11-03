package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/goccy/go-yaml"
)

// decodeAgentFrontmatter is the single AGENT.md frontmatter decoder, shared by parse and edit. It
// canonicalizes the kind and reports whether the v0.5.0 stays_on alias was used.
func decodeAgentFrontmatter(data []byte, parsed *parsedAgentDef) (bool, error) {
	if hasEmbeddedTabFrontmatterKey(data) {
		return false, ErrInvalidAgentFrontmatterKey
	}
	data, err := ignoreRetiredAgentHookMatchers(data)
	if err != nil {
		return false, err
	}

	yamlErr := yaml.UnmarshalWithOptions(data, parsed, yaml.Strict())
	if yamlErr == nil {
		kind, deprecated := CanonicalAgentKind(parsed.Kind)
		parsed.Kind = kind
		return deprecated, nil
	}

	var parsedTOML parsedAgentDef
	meta, tomlErr := toml.Decode(string(data), &parsedTOML)
	if tomlErr != nil {
		return false, fmt.Errorf(
			"decode agent frontmatter: %w",
			errors.Join(
				fmt.Errorf("yaml: %w", yamlErr),
				fmt.Errorf("toml: %w", tomlErr),
			),
		)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return false, fmt.Errorf("decode agent frontmatter: unknown field %q", undecoded[0].String())
	}
	*parsed = parsedTOML
	kind, deprecated := CanonicalAgentKind(parsed.Kind)
	parsed.Kind = kind
	return deprecated, nil
}

var agentKindAliasWarnings sync.Map

// warnAgentKindAlias logs the stays_on deprecation once per AGENT.md path per process.
// Removal: v0.5.2.
func warnAgentKindAlias(path string) {
	path = filepath.Clean(path)
	if _, loaded := agentKindAliasWarnings.LoadOrStore(path, struct{}{}); !loaded {
		DeprecationLogger().Warn(
			"agent kind stays_on is deprecated; use always_on (stays_on is removed in v0.5.2)",
			"path", path,
		)
	}
}
