package agent

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/compozy/compozy/internal/core/model"
)

type catalogSnapshot struct {
	specs map[string]Spec
	order []string
}

var (
	activeCatalogMu sync.RWMutex
	activeCatalog   *catalogSnapshot
)

// OverlayEntry captures one declarative ACP runtime overlay entry assembled during command bootstrap.
type OverlayEntry struct {
	Name               string
	DisplayName        string
	Command            string
	SetupAgentName     string
	DefaultModel       string
	SupportsAddDirs    *bool
	UsesBootstrapModel *bool
	DocsURL            string
	InstallHint        string
	FullAccessModeID   string
	FixedArgs          []string
	ProbeArgs          []string
	EnvVars            map[string]string
	Fallbacks          []Launcher
	Bootstrap          OverlayBootstrap
	Metadata           map[string]string
}

// OverlayBootstrap declares typed ACP bootstrap flags for declarative IDE overlays.
type OverlayBootstrap struct {
	ModelFlag             string
	ReasoningEffortFlag   string
	AddDirFlag            string
	DefaultAccessModeArgs []string
	FullAccessModeArgs    []string
}

// ActivateOverlay installs one command-scoped ACP runtime overlay built from
// extension-declared IDE providers and returns a restore function.
func ActivateOverlay(entries []OverlayEntry) (func(), error) {
	snapshot, err := buildOverlayCatalog(entries)
	if err != nil {
		return nil, err
	}

	activeCatalogMu.Lock()
	previous := activeCatalog
	activeCatalog = snapshot
	activeCatalogMu.Unlock()

	return func() {
		activeCatalogMu.Lock()
		activeCatalog = previous
		activeCatalogMu.Unlock()
	}, nil
}

func buildOverlayCatalog(entries []OverlayEntry) (*catalogSnapshot, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	snapshot := baseCatalogSnapshot()
	added := make([]string, 0)
	for i := range entries {
		spec, err := specFromDeclaredIDEProvider(entries[i])
		if err != nil {
			return nil, err
		}
		if _, ok := snapshot.specs[spec.ID]; !ok {
			added = append(added, spec.ID)
		}
		snapshot.specs[spec.ID] = spec
	}

	slices.Sort(added)
	snapshot.order = append(snapshot.order, added...)
	return &snapshot, nil
}

func currentCatalogSnapshot() catalogSnapshot {
	activeCatalogMu.RLock()
	if activeCatalog != nil {
		snapshot := cloneCatalogSnapshot(*activeCatalog)
		activeCatalogMu.RUnlock()
		return snapshot
	}
	activeCatalogMu.RUnlock()

	return baseCatalogSnapshot()
}

func baseCatalogSnapshot() catalogSnapshot {
	registryMu.RLock()
	defer registryMu.RUnlock()

	specs := make(map[string]Spec, len(registry))
	for ide := range registry {
		spec := registry[ide]
		specs[ide] = cloneAgentSpec(spec)
	}
	return catalogSnapshot{
		specs: specs,
		order: append([]string(nil), supportedRegistryIDEOrder...),
	}
}

func cloneCatalogSnapshot(snapshot catalogSnapshot) catalogSnapshot {
	specs := make(map[string]Spec, len(snapshot.specs))
	for ide := range snapshot.specs {
		spec := snapshot.specs[ide]
		specs[ide] = cloneAgentSpec(spec)
	}
	return catalogSnapshot{
		specs: specs,
		order: append([]string(nil), snapshot.order...),
	}
}

func specFromDeclaredIDEProvider(entry OverlayEntry) (Spec, error) {
	id := normalizeOverlayIdentifier(entry.Name)
	if id == "" {
		return Spec{}, fmt.Errorf("declare ACP runtime overlay: provider name is required")
	}

	command, fixedArgs, err := splitOverlayCommand(entry.Command)
	if err != nil {
		return Spec{}, fmt.Errorf("declare ACP runtime overlay %q: %w", entry.Name, err)
	}

	metadataFixedArgs, err := parseOverlayArgs(entry.Metadata["fixed_args"])
	if err != nil {
		return Spec{}, fmt.Errorf("declare ACP runtime overlay %q fixed_args: %w", entry.Name, err)
	}
	if len(entry.FixedArgs) > 0 {
		fixedArgs = slices.Clone(entry.FixedArgs)
	} else if len(metadataFixedArgs) > 0 {
		fixedArgs = metadataFixedArgs
	}
	probeArgs, err := parseOverlayArgs(entry.Metadata["probe_args"])
	if err != nil {
		return Spec{}, fmt.Errorf("declare ACP runtime overlay %q probe_args: %w", entry.Name, err)
	}
	if len(entry.ProbeArgs) > 0 {
		probeArgs = slices.Clone(entry.ProbeArgs)
	}

	usesBootstrapModel := overlayBoolOrDefault(
		entry.UsesBootstrapModel,
		entry.Metadata["uses_bootstrap_model"],
		strings.TrimSpace(entry.Bootstrap.ModelFlag) != "",
	)
	supportsAddDirs := overlayBoolOrDefault(
		entry.SupportsAddDirs,
		entry.Metadata["supports_add_dirs"],
		strings.TrimSpace(entry.Bootstrap.AddDirFlag) != "",
	)

	spec := Spec{
		ID: id,
		DisplayName: overlayFirstNonEmpty(
			strings.TrimSpace(entry.DisplayName),
			strings.TrimSpace(entry.Metadata["display_name"]),
			strings.TrimSpace(entry.Name),
		),
		SetupAgentName: overlayFirstNonEmpty(
			strings.TrimSpace(entry.SetupAgentName),
			strings.TrimSpace(entry.Metadata["agent_name"]),
		),
		DefaultModel: overlayFirstNonEmpty(
			strings.TrimSpace(entry.DefaultModel),
			strings.TrimSpace(entry.Metadata["default_model"]),
			model.DefaultCodexModel,
		),
		Command:            command,
		FixedArgs:          fixedArgs,
		ProbeArgs:          probeArgs,
		SupportsAddDirs:    supportsAddDirs,
		UsesBootstrapModel: usesBootstrapModel,
		DocsURL: overlayFirstNonEmpty(
			strings.TrimSpace(entry.DocsURL),
			strings.TrimSpace(entry.Metadata["docs_url"]),
		),
		InstallHint: overlayFirstNonEmpty(
			strings.TrimSpace(entry.InstallHint),
			strings.TrimSpace(entry.Metadata["install_hint"]),
		),
		FullAccessModeID: overlayFirstNonEmpty(
			strings.TrimSpace(entry.FullAccessModeID),
			strings.TrimSpace(entry.Metadata["full_access_mode_id"]),
		),
		EnvVars:       overlayEnv(entry.EnvVars, entry.Metadata),
		Fallbacks:     cloneLaunchers(entry.Fallbacks),
		BootstrapArgs: overlayBootstrapArgs(entry.Bootstrap, usesBootstrapModel),
	}
	if strings.TrimSpace(spec.DisplayName) == "" {
		spec.DisplayName = spec.ID
	}
	return spec, nil
}

func normalizeOverlayIdentifier(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func splitOverlayCommand(raw string) (string, []string, error) {
	parts, err := splitOverlayWords(raw)
	if err != nil {
		return "", nil, err
	}
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("command is required")
	}
	return parts[0], parts[1:], nil
}

func parseOverlayArgs(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	return splitOverlayWords(trimmed)
}

func parseOverlayBool(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	parsed, err := strconv.ParseBool(trimmed)
	return err == nil && parsed
}

func parseOverlayEnv(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}

	env := make(map[string]string)
	for key, value := range metadata {
		envKey, ok := strings.CutPrefix(key, "env.")
		if !ok {
			continue
		}
		if trimmedKey := strings.TrimSpace(envKey); trimmedKey != "" {
			env[trimmedKey] = value
		}
	}
	if len(env) == 0 {
		return nil
	}
	return env
}

func overlayEnv(explicit map[string]string, metadata map[string]string) map[string]string {
	if len(explicit) > 0 {
		return mapsClone(explicit)
	}
	return parseOverlayEnv(metadata)
}

func overlayBoolOrDefault(explicit *bool, legacy string, fallback bool) bool {
	if explicit != nil {
		return *explicit
	}
	if strings.TrimSpace(legacy) != "" {
		return parseOverlayBool(legacy)
	}
	return fallback
}

func overlayBootstrapArgs(
	bootstrap OverlayBootstrap,
	usesBootstrapModel bool,
) func(modelName, reasoningEffort string, addDirs []string, accessMode string) []string {
	if strings.TrimSpace(bootstrap.ModelFlag) == "" &&
		strings.TrimSpace(bootstrap.ReasoningEffortFlag) == "" &&
		strings.TrimSpace(bootstrap.AddDirFlag) == "" &&
		len(bootstrap.DefaultAccessModeArgs) == 0 &&
		len(bootstrap.FullAccessModeArgs) == 0 {
		return nil
	}

	return func(modelName, reasoningEffort string, addDirs []string, accessMode string) []string {
		args := make(
			[]string,
			0,
			len(bootstrap.DefaultAccessModeArgs)+len(bootstrap.FullAccessModeArgs)+(len(addDirs)*2)+4,
		)
		args = append(args, slices.Clone(bootstrap.DefaultAccessModeArgs)...)
		if accessMode == model.AccessModeFull {
			args = append(args, slices.Clone(bootstrap.FullAccessModeArgs)...)
		}
		if usesBootstrapModel && strings.TrimSpace(bootstrap.ModelFlag) != "" && strings.TrimSpace(modelName) != "" {
			args = append(args, strings.TrimSpace(bootstrap.ModelFlag), modelName)
		}
		if strings.TrimSpace(bootstrap.ReasoningEffortFlag) != "" && strings.TrimSpace(reasoningEffort) != "" {
			args = append(args, strings.TrimSpace(bootstrap.ReasoningEffortFlag), reasoningEffort)
		}
		if strings.TrimSpace(bootstrap.AddDirFlag) != "" {
			flag := strings.TrimSpace(bootstrap.AddDirFlag)
			for _, dir := range addDirs {
				if trimmed := strings.TrimSpace(dir); trimmed != "" {
					args = append(args, flag, trimmed)
				}
			}
		}
		return args
	}
}

func overlayFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func splitOverlayWords(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	parser := overlayWordParser{parts: make([]string, 0)}
	for _, r := range trimmed {
		if parser.handleEscapedRune(r) {
			continue
		}
		if parser.handleEscapeStart(r) {
			continue
		}
		if parser.handleQuoteRune(r) {
			continue
		}
		if parser.handleWhitespace(r) {
			continue
		}
		parser.current.WriteRune(r)
	}

	return parser.finish()
}

type overlayWordParser struct {
	parts                []string
	current              strings.Builder
	inSingleQuote        bool
	inDoubleQuote        bool
	escaped              bool
	escapedInDoubleQuote bool
}

func (p *overlayWordParser) handleEscapedRune(r rune) bool {
	if !p.escaped {
		return false
	}
	if p.escapedInDoubleQuote && !isDoubleQuoteEscapable(r) {
		p.current.WriteRune('\\')
	}
	p.current.WriteRune(r)
	p.escaped = false
	p.escapedInDoubleQuote = false
	return true
}

func (p *overlayWordParser) handleEscapeStart(r rune) bool {
	if r != '\\' {
		return false
	}
	if p.inSingleQuote {
		p.current.WriteRune(r)
		return true
	}
	p.escaped = true
	p.escapedInDoubleQuote = p.inDoubleQuote
	return true
}

func isDoubleQuoteEscapable(r rune) bool {
	switch r {
	case '"', '\\', '$', '`':
		return true
	default:
		return false
	}
}

func (p *overlayWordParser) handleQuoteRune(r rune) bool {
	switch r {
	case '\'':
		if p.inDoubleQuote {
			p.current.WriteRune(r)
			return true
		}
		p.inSingleQuote = !p.inSingleQuote
		return true
	case '"':
		if p.inSingleQuote {
			p.current.WriteRune(r)
			return true
		}
		p.inDoubleQuote = !p.inDoubleQuote
		return true
	default:
		return false
	}
}

func (p *overlayWordParser) handleWhitespace(r rune) bool {
	if !unicode.IsSpace(r) {
		return false
	}
	if p.inSingleQuote || p.inDoubleQuote {
		p.current.WriteRune(r)
		return true
	}
	p.flushCurrent()
	return true
}

func (p *overlayWordParser) flushCurrent() {
	if p.current.Len() == 0 {
		return
	}
	p.parts = append(p.parts, p.current.String())
	p.current.Reset()
}

func (p *overlayWordParser) finish() ([]string, error) {
	if p.escaped {
		return nil, fmt.Errorf("unterminated escape")
	}
	if p.inSingleQuote || p.inDoubleQuote {
		return nil, fmt.Errorf("unterminated quote")
	}
	p.flushCurrent()
	return p.parts, nil
}
