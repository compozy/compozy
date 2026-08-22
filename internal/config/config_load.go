package config

import (
	"fmt"

	"os"
	"strings"
)

type loadOptions struct {
	workspaceRoot             string
	profileName               string
	skipDotEnv                bool
	skipValidate              bool
	skipWorkspaceOverlayFiles bool
}

type envLookup func(string) (string, bool)

func processEnvLookup(key string) (string, bool) {
	return os.LookupEnv(key)
}

func layeredEnvLookup(primary envLookup, fallback envLookup) envLookup {
	return func(key string) (string, bool) {
		if primary != nil {
			if value, ok := primary(key); ok {
				return value, true
			}
		}
		if fallback != nil {
			return fallback(key)
		}
		return "", false
	}
}

// LoadOption customizes configuration loading.
type LoadOption func(*loadOptions)

// WithWorkspaceRoot loads the optional workspace overlay from `<root>/.compozy/config.toml`.
// When omitted, Load applies only the built-in defaults and the global Compozy home config.
func WithWorkspaceRoot(root string) LoadOption {
	return func(opts *loadOptions) {
		opts.workspaceRoot = root
	}
}

// WithProfile loads the personal and workspace layers bound to one active profile name.
func WithProfile(name string) LoadOption {
	return func(opts *loadOptions) {
		opts.profileName = name
	}
}

// WithoutWorkspaceOverlayFiles keeps workspace environment lookup while omitting workspace config files.
func WithoutWorkspaceOverlayFiles() LoadOption {
	return func(opts *loadOptions) {
		opts.skipWorkspaceOverlayFiles = true
	}
}

func withoutDotEnv() LoadOption {
	return func(opts *loadOptions) {
		opts.skipDotEnv = true
	}
}

func withoutValidation() LoadOption {
	return func(opts *loadOptions) {
		opts.skipValidate = true
	}
}

// Load reads the default config, the optional global config, and the optional workspace overlay.
// Workspace overlays are loaded only when WithWorkspaceRoot supplies an explicit root.
func Load(opts ...LoadOption) (Config, error) {
	options := loadOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	workspaceRoot, err := resolveWorkspaceRoot(options.workspaceRoot)
	if err != nil {
		return Config{}, err
	}

	lookup := processEnvLookup
	if !options.skipDotEnv {
		dotenvLookup, err := loadDotEnvLookup(workspaceRoot)
		if err != nil {
			return Config{}, err
		}
		lookup = layeredEnvLookup(processEnvLookup, dotenvLookup)
	}

	homePaths, err := resolveHomePaths(lookup)
	if err != nil {
		return Config{}, err
	}

	return loadWithHome(
		homePaths,
		workspaceRoot,
		options.profileName,
		options.skipWorkspaceOverlayFiles,
		options.skipValidate,
		lookup,
	)
}

// LoadForHome reads the default config, the optional global config, and the optional workspace
// overlay using the supplied Compozy home layout instead of the ambient process home.
func LoadForHome(homePaths HomePaths, opts ...LoadOption) (Config, error) {
	options := loadOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	workspaceRoot, err := resolveWorkspaceRoot(options.workspaceRoot)
	if err != nil {
		return Config{}, err
	}

	lookup := processEnvLookup
	if !options.skipDotEnv {
		dotenvLookup, err := loadDotEnvLookup(workspaceRoot)
		if err != nil {
			return Config{}, err
		}
		lookup = layeredEnvLookup(processEnvLookup, dotenvLookup)
	}

	return loadWithHome(
		homePaths,
		workspaceRoot,
		options.profileName,
		options.skipWorkspaceOverlayFiles,
		options.skipValidate,
		lookup,
	)
}

func loadWithHome(
	homePaths HomePaths,
	workspaceRoot string,
	profileName string,
	skipWorkspaceOverlayFiles bool,
	skipValidate bool,
	lookup envLookup,
) (Config, error) {
	cfg := DefaultWithHome(homePaths)
	if err := ApplyConfigOverlayFile(homePaths.ConfigFile, &cfg); err != nil {
		return Config{}, fmt.Errorf("load global config: %w", err)
	}
	if err := applyConfigMCPSidecarFile(globalMCPJSONFile(homePaths), &cfg); err != nil {
		return Config{}, fmt.Errorf("load global MCP JSON: %w", err)
	}
	profileName = strings.TrimSpace(profileName)
	if profileName != "" {
		if err := ValidateResourceProfileName(profileName); err != nil {
			return Config{}, fmt.Errorf("load profile config: %w", err)
		}
		if err := applyProfileConfigOverlayFile(profileConfigFile(homePaths, profileName), &cfg, RoleFieldSourceProfile); err != nil {
			return Config{}, fmt.Errorf("load profile config: %w", err)
		}
		if err := applyConfigMCPSidecarFile(profileMCPJSONFile(homePaths, profileName), &cfg); err != nil {
			return Config{}, fmt.Errorf("load profile MCP JSON: %w", err)
		}
	}
	if !skipWorkspaceOverlayFiles && hasDistinctWorkspaceOverlay(homePaths, workspaceRoot) {
		if err := applyWorkspaceConfigOverlayFile(workspaceConfigFile(workspaceRoot), &cfg); err != nil {
			return Config{}, fmt.Errorf("load workspace config: %w", err)
		}
		if err := applyConfigMCPSidecarFile(workspaceMCPJSONFile(workspaceRoot), &cfg); err != nil {
			return Config{}, fmt.Errorf("load workspace MCP JSON: %w", err)
		}
		if profileName != "" {
			if err := applyProfileConfigOverlayFile(
				workspaceProfileConfigFile(workspaceRoot, profileName),
				&cfg,
				RoleFieldSourceWorkspaceProfile,
			); err != nil {
				return Config{}, fmt.Errorf("load workspace profile config: %w", err)
			}
			if err := applyConfigMCPSidecarFile(workspaceProfileMCPJSONFile(workspaceRoot, profileName), &cfg); err != nil {
				return Config{}, fmt.Errorf("load workspace profile MCP JSON: %w", err)
			}
		}
	}
	if err := normalizeConfigPaths(&cfg); err != nil {
		return Config{}, err
	}
	ApplyMarketplaceCatalogEnvironment(&cfg, func(key string) string {
		value, _ := lookup(key)
		return value
	})

	if !skipValidate {
		if err := cfg.validateWithEnv(lookup); err != nil {
			return Config{}, fmt.Errorf("validate config: %w", err)
		}
	}

	return cfg, nil
}
