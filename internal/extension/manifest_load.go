package extensionpkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/compozy/agh/internal/extension/surfaces"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/modelcatalog"
)

// LoadManifest reads one extension manifest from dir, preferring TOML over JSON.
func LoadManifest(dir string) (*Manifest, error) {
	manifestDir := strings.TrimSpace(dir)
	if manifestDir == "" {
		return nil, &ManifestValidationError{
			Field:   "dir",
			Message: "directory is required",
		}
	}

	tomlPath := filepath.Join(manifestDir, manifestTOMLFileName)
	if exists, err := fileExists(tomlPath); err != nil {
		return nil, fmt.Errorf("extension: stat %q: %w", tomlPath, err)
	} else if exists {
		return loadManifestTOML(tomlPath)
	}

	jsonPath := filepath.Join(manifestDir, manifestJSONFileName)
	if exists, err := fileExists(jsonPath); err != nil {
		return nil, fmt.Errorf("extension: stat %q: %w", jsonPath, err)
	} else if exists {
		return loadManifestJSON(jsonPath)
	}

	return nil, &ManifestNotFoundError{
		Dir:   manifestDir,
		Paths: []string{tomlPath, jsonPath},
	}
}

// Validate checks the manifest schema and daemon compatibility.
func (m *Manifest) Validate() error {
	if err := requireField(manifestNameKey, m.Name); err != nil {
		return err
	}
	if err := requireField("version", m.Version); err != nil {
		return err
	}
	if err := validateSemanticVersionField("version", m.Version); err != nil {
		return err
	}
	if err := requireField("min_agh_version", m.MinAGHVersion); err != nil {
		return err
	}
	if err := validateSemanticVersionField("min_agh_version", m.MinAGHVersion); err != nil {
		return err
	}
	if err := validateDaemonCompatibility(m.MinAGHVersion); err != nil {
		return err
	}
	if err := validateEnvRequirements("requires_env", m.RequiresEnv); err != nil {
		return err
	}
	if err := m.NetworkParticipation.Validate("network_participation"); err != nil {
		return err
	}
	if err := validateManifestEnvMaps(
		"subprocess",
		"extensions",
		m.Subprocess.Env,
		m.Subprocess.SecretEnv,
	); err != nil {
		return err
	}
	if err := validateManifestHookEnv(m.Resources.Hooks); err != nil {
		return err
	}
	if err := validateManifestMCPServerEnv(m.Resources.MCPServers); err != nil {
		return err
	}
	if err := validateToolConfigs(m.Name, m.Resources.Tools); err != nil {
		return err
	}
	if err := validateDottedIdentifiers("capabilities.provides", m.Capabilities.Provides, false); err != nil {
		return err
	}
	if err := m.validateModelSourceCapability(); err != nil {
		return err
	}
	if err := validateSlashIdentifiers("actions.requires", m.Actions.Requires); err != nil {
		return err
	}
	if err := validateDottedIdentifiers("security.capabilities", m.Security.Capabilities, true); err != nil {
		return err
	}
	if err := m.validateBridgeAdapterCapability(); err != nil {
		return err
	}
	if _, err := surfaces.ResolveManifestRequest(
		m.Resources.Publish.Families,
		m.Resources.Publish.MaxScope,
	); err != nil {
		return &ManifestValidationError{
			Field:   manifestResourcesPublishPath,
			Message: err.Error(),
		}
	}
	return nil
}

func (m *Manifest) validateModelSourceCapability() error {
	if !providesCapability(m.Capabilities.Provides, extensionprotocol.CapabilityProvideModelSource) {
		return nil
	}
	if _, err := modelcatalog.SourceKindExtensionID(m.Name); err != nil {
		return &ManifestValidationError{
			Field:   manifestNameKey,
			Value:   m.Name,
			Message: err.Error(),
		}
	}
	return nil
}

func (m *Manifest) validateBridgeAdapterCapability() error {
	if !providesCapability(m.Capabilities.Provides, extensionprotocol.CapabilityProvideBridgeAdapter) {
		return nil
	}
	if err := requireField("bridge.platform", m.Bridge.Platform); err != nil {
		return err
	}
	if err := requireField("bridge.display_name", m.Bridge.DisplayName); err != nil {
		return err
	}
	if err := validateBridgeSecretSlots(m.Bridge.SecretSlots); err != nil {
		return err
	}
	if m.Bridge.ConfigSchema == nil {
		return nil
	}
	if err := m.Bridge.ConfigSchema.Validate(); err != nil {
		return &ManifestValidationError{
			Field:   "bridge.config_schema",
			Message: err.Error(),
		}
	}
	return nil
}

// IsZero reports whether the duration is unset.
func (d Duration) IsZero() bool {
	return time.Duration(d) == 0
}

// String returns the canonical duration string.
func (d Duration) String() string {
	return time.Duration(d).String()
}

// MarshalJSON emits the duration as a quoted duration string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// MarshalText emits the duration as text.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalJSON accepts duration strings and integer nanoseconds.
func (d *Duration) UnmarshalJSON(data []byte) error {
	if string(data) == manifestNullKey {
		*d = 0
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		return d.UnmarshalText([]byte(text))
	}

	var nanos int64
	if err := json.Unmarshal(data, &nanos); err == nil {
		*d = Duration(time.Duration(nanos))
		return nil
	}

	return fmt.Errorf("extension: invalid duration %s", string(data))
}

// UnmarshalText parses duration strings like "30s".
func (d *Duration) UnmarshalText(text []byte) error {
	trimmed := strings.TrimSpace(string(text))
	if trimmed == "" {
		*d = 0
		return nil
	}

	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return fmt.Errorf("extension: parse duration %q: %w", trimmed, err)
	}
	*d = Duration(parsed)
	return nil
}

func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if info.IsDir() {
		return false, fmt.Errorf("%q is a directory", path)
	}
	return true, nil
}

func loadManifestTOML(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("extension: read manifest %q: %w", path, err)
	}

	var doc manifestDocument
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return nil, fmt.Errorf("extension: decode manifest %q: %w", path, err)
	}

	manifest, err := doc.toManifest()
	if err != nil {
		return nil, err
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func loadManifestJSON(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("extension: read manifest %q: %w", path, err)
	}

	var doc manifestDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("extension: decode manifest %q: %w", path, err)
	}

	manifest, err := doc.toManifest()
	if err != nil {
		return nil, err
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return &manifest, nil
}
