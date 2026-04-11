package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// ManifestFormat identifies the on-disk encoding used for a manifest file.
type ManifestFormat string

const (
	manifestFormatTOML ManifestFormat = "toml"
	manifestFormatJSON ManifestFormat = "json"
)

// ManifestNotFoundError reports that neither supported manifest file exists in the extension directory.
type ManifestNotFoundError struct {
	Dir            string
	CandidatePaths []string
}

func (e *ManifestNotFoundError) Error() string {
	if e == nil {
		return "extension manifest not found"
	}

	return fmt.Sprintf(
		"extension manifest not found in %q (tried %s)",
		e.Dir,
		strings.Join(e.CandidatePaths, ", "),
	)
}

// ManifestDecodeError reports that a manifest file could not be decoded.
type ManifestDecodeError struct {
	Path   string
	Format ManifestFormat
	Err    error
}

func (e *ManifestDecodeError) Error() string {
	if e == nil {
		return "decode extension manifest"
	}

	return fmt.Sprintf("decode %s manifest %q: %v", e.Format, e.Path, e.Err)
}

func (e *ManifestDecodeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ManifestValidationError reports that a decoded manifest failed validation.
type ManifestValidationError struct {
	Path string
	Err  error
}

func (e *ManifestValidationError) Error() string {
	if e == nil {
		return "validate extension manifest"
	}

	return fmt.Sprintf("validate extension manifest %q: %v", e.Path, e.Err)
}

func (e *ManifestValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// LoadManifest loads, parses, and validates an extension manifest from dir.
func LoadManifest(ctx context.Context, dir string) (*Manifest, error) {
	if err := contextError(ctx, "load extension manifest"); err != nil {
		return nil, err
	}

	resolvedDir := strings.TrimSpace(dir)
	if resolvedDir == "" {
		return nil, fmt.Errorf("load extension manifest: directory is empty")
	}

	tomlPath := filepath.Join(resolvedDir, ManifestFileNameTOML)
	jsonPath := filepath.Join(resolvedDir, ManifestFileNameJSON)

	if _, err := os.Stat(tomlPath); err == nil {
		manifest, loadErr := loadManifestFile(ctx, tomlPath, manifestFormatTOML)
		if loadErr != nil {
			return nil, loadErr
		}
		if _, err := os.Stat(jsonPath); err == nil {
			slog.Warn(
				"extension.toml takes precedence over extension.json",
				slog.String("dir", resolvedDir),
				slog.String("manifest_path", tomlPath),
				slog.String("ignored_manifest_path", jsonPath),
			)
		}
		return manifest, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat extension manifest %q: %w", tomlPath, err)
	}

	if _, err := os.Stat(jsonPath); err == nil {
		return loadManifestFile(ctx, jsonPath, manifestFormatJSON)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat extension manifest %q: %w", jsonPath, err)
	}

	return nil, &ManifestNotFoundError{
		Dir:            resolvedDir,
		CandidatePaths: []string{tomlPath, jsonPath},
	}
}

func loadManifestFile(
	ctx context.Context,
	path string,
	format ManifestFormat,
) (*Manifest, error) {
	if err := contextError(ctx, "load extension manifest file"); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read extension manifest %q: %w", path, err)
	}

	raw, err := decodeRawManifest(data, format)
	if err != nil {
		return nil, &ManifestDecodeError{Path: path, Format: format, Err: err}
	}

	if err := raw.validatePresence(); err != nil {
		return nil, &ManifestValidationError{Path: path, Err: err}
	}

	manifest := raw.toManifest()
	if err := ValidateManifest(ctx, manifest); err != nil {
		return nil, &ManifestValidationError{Path: path, Err: err}
	}

	return manifest, nil
}

func decodeRawManifest(data []byte, format ManifestFormat) (rawManifest, error) {
	var raw rawManifest

	switch format {
	case manifestFormatTOML:
		decoder := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields()
		if err := decoder.Decode(&raw); err != nil {
			return rawManifest{}, err
		}
		return raw, nil
	case manifestFormatJSON:
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&raw); err != nil {
			return rawManifest{}, err
		}
		if err := ensureJSONEOF(decoder); err != nil {
			return rawManifest{}, err
		}
		return raw, nil
	default:
		return rawManifest{}, fmt.Errorf("unsupported manifest format %q", format)
	}
}

func ensureJSONEOF(decoder *json.Decoder) error {
	if decoder == nil {
		return fmt.Errorf("json decoder is nil")
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	return fmt.Errorf("unexpected trailing JSON content")
}

func contextError(ctx context.Context, action string) error {
	if ctx == nil {
		return nil
	}
	if err := context.Cause(ctx); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}

type rawManifest struct {
	Extension  *rawExtensionInfo    `toml:"extension"  json:"extension"`
	Subprocess *rawSubprocessConfig `toml:"subprocess" json:"subprocess"`
	Security   *rawSecurityConfig   `toml:"security"   json:"security"`
	Hooks      []rawHookDeclaration `toml:"hooks"      json:"hooks"`
	Resources  *rawResourcesConfig  `toml:"resources"  json:"resources"`
	Providers  *rawProvidersConfig  `toml:"providers"  json:"providers"`
}

func (r rawManifest) validatePresence() error {
	if r.Extension == nil {
		return newManifestFieldError("extension", "", "section is required")
	}
	if r.Security == nil {
		return newManifestFieldError("security", "", "section is required")
	}
	return nil
}

func (r rawManifest) toManifest() *Manifest {
	manifest := &Manifest{
		Extension: ExtensionInfo{},
		Security:  SecurityConfig{},
	}
	if r.Extension != nil {
		manifest.Extension = r.Extension.toExtensionInfo()
	}
	if r.Subprocess != nil {
		subprocess := r.Subprocess.toSubprocessConfig()
		manifest.Subprocess = &subprocess
	}
	if r.Security != nil {
		manifest.Security = r.Security.toSecurityConfig()
	}
	for _, hook := range r.Hooks {
		manifest.Hooks = append(manifest.Hooks, hook.toHookDeclaration())
	}
	if r.Resources != nil {
		manifest.Resources = r.Resources.toResourcesConfig()
	}
	if r.Providers != nil {
		manifest.Providers = r.Providers.toProvidersConfig()
	}
	return manifest
}

type rawExtensionInfo struct {
	Name              string `toml:"name"                json:"name"`
	Version           string `toml:"version"             json:"version"`
	Description       string `toml:"description"         json:"description"`
	MinCompozyVersion string `toml:"min_compozy_version" json:"min_compozy_version"`
}

func (r rawExtensionInfo) toExtensionInfo() ExtensionInfo {
	return ExtensionInfo{
		Name:              strings.TrimSpace(r.Name),
		Version:           strings.TrimSpace(r.Version),
		Description:       strings.TrimSpace(r.Description),
		MinCompozyVersion: strings.TrimSpace(r.MinCompozyVersion),
	}
}

type rawSubprocessConfig struct {
	Command           string            `toml:"command"             json:"command"`
	Args              []string          `toml:"args"                json:"args"`
	Env               map[string]string `toml:"env"                 json:"env"`
	ShutdownTimeout   durationValue     `toml:"shutdown_timeout"    json:"shutdown_timeout"`
	HealthCheckPeriod durationValue     `toml:"health_check_period" json:"health_check_period"`
}

func (r rawSubprocessConfig) toSubprocessConfig() SubprocessConfig {
	return SubprocessConfig{
		Command:           strings.TrimSpace(r.Command),
		Args:              cloneStrings(r.Args),
		Env:               cloneStringMap(r.Env),
		ShutdownTimeout:   r.ShutdownTimeout.Duration,
		HealthCheckPeriod: r.HealthCheckPeriod.Duration,
	}
}

type rawSecurityConfig struct {
	Capabilities []Capability `toml:"capabilities" json:"capabilities"`
}

func (r rawSecurityConfig) toSecurityConfig() SecurityConfig {
	capabilities := make([]Capability, 0, len(r.Capabilities))
	for _, capability := range r.Capabilities {
		capabilities = append(capabilities, Capability(strings.TrimSpace(string(capability))))
	}
	return SecurityConfig{Capabilities: capabilities}
}

type rawHookDeclaration struct {
	Event    HookName      `toml:"event"    json:"event"`
	Priority *int          `toml:"priority" json:"priority"`
	Required bool          `toml:"required" json:"required"`
	Timeout  durationValue `toml:"timeout"  json:"timeout"`
}

func (r rawHookDeclaration) toHookDeclaration() HookDeclaration {
	priority := DefaultHookPriority
	if r.Priority != nil {
		priority = *r.Priority
	}
	return HookDeclaration{
		Event:    HookName(strings.TrimSpace(string(r.Event))),
		Priority: priority,
		Required: r.Required,
		Timeout:  r.Timeout.Duration,
	}
}

type rawResourcesConfig struct {
	Skills []string `toml:"skills" json:"skills"`
}

func (r rawResourcesConfig) toResourcesConfig() ResourcesConfig {
	return ResourcesConfig{Skills: cloneStrings(r.Skills)}
}

type rawProvidersConfig struct {
	IDE    []rawProviderEntry `toml:"ide"    json:"ide"`
	Review []rawProviderEntry `toml:"review" json:"review"`
	Model  []rawProviderEntry `toml:"model"  json:"model"`
}

func (r rawProvidersConfig) toProvidersConfig() ProvidersConfig {
	return ProvidersConfig{
		IDE:    toProviderEntries(r.IDE),
		Review: toProviderEntries(r.Review),
		Model:  toProviderEntries(r.Model),
	}
}

type rawProviderEntry struct {
	Name               string                `toml:"name"                 json:"name"`
	Kind               ProviderKind          `toml:"kind"                 json:"kind"`
	Target             string                `toml:"target"               json:"target"`
	Command            string                `toml:"command"              json:"command"`
	DisplayName        string                `toml:"display_name"         json:"display_name"`
	SetupAgentName     string                `toml:"setup_agent_name"     json:"setup_agent_name"`
	DefaultModel       string                `toml:"default_model"        json:"default_model"`
	SupportsAddDirs    *bool                 `toml:"supports_add_dirs"    json:"supports_add_dirs"`
	UsesBootstrapModel *bool                 `toml:"uses_bootstrap_model" json:"uses_bootstrap_model"`
	DocsURL            string                `toml:"docs_url"             json:"docs_url"`
	InstallHint        string                `toml:"install_hint"         json:"install_hint"`
	FullAccessModeID   string                `toml:"full_access_mode_id"  json:"full_access_mode_id"`
	FixedArgs          []string              `toml:"fixed_args"           json:"fixed_args"`
	ProbeArgs          []string              `toml:"probe_args"           json:"probe_args"`
	Env                map[string]string     `toml:"env"                  json:"env"`
	Fallbacks          []rawProviderLauncher `toml:"fallbacks"            json:"fallbacks"`
	Bootstrap          *rawProviderBootstrap `toml:"bootstrap"            json:"bootstrap"`
	Metadata           map[string]string     `toml:"metadata"             json:"metadata"`
}

type rawProviderLauncher struct {
	Command   string   `toml:"command"    json:"command"`
	FixedArgs []string `toml:"fixed_args" json:"fixed_args"`
	ProbeArgs []string `toml:"probe_args" json:"probe_args"`
}

type rawProviderBootstrap struct {
	ModelFlag             string   `toml:"model_flag"               json:"model_flag"`
	ReasoningEffortFlag   string   `toml:"reasoning_effort_flag"    json:"reasoning_effort_flag"`
	AddDirFlag            string   `toml:"add_dir_flag"             json:"add_dir_flag"`
	DefaultAccessModeArgs []string `toml:"default_access_mode_args" json:"default_access_mode_args"`
	FullAccessModeArgs    []string `toml:"full_access_mode_args"    json:"full_access_mode_args"`
}

func toProviderEntries(raw []rawProviderEntry) []ProviderEntry {
	entries := make([]ProviderEntry, 0, len(raw))
	for i := range raw {
		entry := raw[i]
		entries = append(entries, ProviderEntry{
			Name:               strings.TrimSpace(entry.Name),
			Kind:               ProviderKind(strings.TrimSpace(string(entry.Kind))),
			Target:             strings.TrimSpace(entry.Target),
			Command:            strings.TrimSpace(entry.Command),
			DisplayName:        strings.TrimSpace(entry.DisplayName),
			SetupAgentName:     strings.TrimSpace(entry.SetupAgentName),
			DefaultModel:       strings.TrimSpace(entry.DefaultModel),
			SupportsAddDirs:    cloneBoolPointer(entry.SupportsAddDirs),
			UsesBootstrapModel: cloneBoolPointer(entry.UsesBootstrapModel),
			DocsURL:            strings.TrimSpace(entry.DocsURL),
			InstallHint:        strings.TrimSpace(entry.InstallHint),
			FullAccessModeID:   strings.TrimSpace(entry.FullAccessModeID),
			FixedArgs:          cloneStrings(entry.FixedArgs),
			ProbeArgs:          cloneStrings(entry.ProbeArgs),
			Env:                cloneStringMap(entry.Env),
			Fallbacks:          toProviderLaunchers(entry.Fallbacks),
			Bootstrap:          toProviderBootstrap(entry.Bootstrap),
			Metadata:           cloneStringMap(entry.Metadata),
		})
	}
	return entries
}

func toProviderLaunchers(raw []rawProviderLauncher) []ProviderLauncher {
	if len(raw) == 0 {
		return nil
	}

	launchers := make([]ProviderLauncher, 0, len(raw))
	for _, launcher := range raw {
		launchers = append(launchers, ProviderLauncher{
			Command:   strings.TrimSpace(launcher.Command),
			FixedArgs: cloneStrings(launcher.FixedArgs),
			ProbeArgs: cloneStrings(launcher.ProbeArgs),
		})
	}
	return launchers
}

func toProviderBootstrap(raw *rawProviderBootstrap) *ProviderBootstrap {
	if raw == nil {
		return nil
	}

	return &ProviderBootstrap{
		ModelFlag:             strings.TrimSpace(raw.ModelFlag),
		ReasoningEffortFlag:   strings.TrimSpace(raw.ReasoningEffortFlag),
		AddDirFlag:            strings.TrimSpace(raw.AddDirFlag),
		DefaultAccessModeArgs: cloneStrings(raw.DefaultAccessModeArgs),
		FullAccessModeArgs:    cloneStrings(raw.FullAccessModeArgs),
	}
}

type durationValue struct {
	time.Duration
}

func (d *durationValue) UnmarshalText(text []byte) error {
	if d == nil {
		return fmt.Errorf("duration value is nil")
	}

	raw := strings.TrimSpace(string(text))
	if raw == "" {
		d.Duration = 0
		return nil
	}

	duration, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", raw, err)
	}
	d.Duration = duration
	return nil
}

func (d *durationValue) UnmarshalJSON(data []byte) error {
	if d == nil {
		return fmt.Errorf("duration value is nil")
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == `""` {
		d.Duration = 0
		return nil
	}

	var rawString string
	if err := json.Unmarshal(data, &rawString); err == nil {
		return d.UnmarshalText([]byte(rawString))
	}

	var rawNumber json.Number
	if err := json.Unmarshal(data, &rawNumber); err == nil {
		value, err := rawNumber.Int64()
		if err != nil {
			return fmt.Errorf("parse numeric duration %q: %w", rawNumber.String(), err)
		}
		d.Duration = time.Duration(value)
		return nil
	}

	return fmt.Errorf("duration must be a string or integer nanoseconds value")
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]string, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, strings.TrimSpace(value))
	}
	return cloned
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return cloned
}

func cloneBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}
