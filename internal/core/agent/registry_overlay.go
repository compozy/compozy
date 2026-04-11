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
	Name     string
	Command  string
	Metadata map[string]string
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
	for _, entry := range entries {
		spec, err := specFromDeclaredIDEProvider(entry)
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
	if len(metadataFixedArgs) > 0 {
		fixedArgs = metadataFixedArgs
	}
	probeArgs, err := parseOverlayArgs(entry.Metadata["probe_args"])
	if err != nil {
		return Spec{}, fmt.Errorf("declare ACP runtime overlay %q probe_args: %w", entry.Name, err)
	}

	spec := Spec{
		ID: id,
		DisplayName: overlayFirstNonEmpty(
			strings.TrimSpace(entry.Metadata["display_name"]),
			strings.TrimSpace(entry.Name),
		),
		SetupAgentName: strings.TrimSpace(entry.Metadata["agent_name"]),
		DefaultModel: overlayFirstNonEmpty(
			strings.TrimSpace(entry.Metadata["default_model"]),
			model.DefaultCodexModel,
		),
		Command:            command,
		FixedArgs:          fixedArgs,
		ProbeArgs:          probeArgs,
		SupportsAddDirs:    parseOverlayBool(entry.Metadata["supports_add_dirs"]),
		UsesBootstrapModel: parseOverlayBool(entry.Metadata["uses_bootstrap_model"]),
		DocsURL:            strings.TrimSpace(entry.Metadata["docs_url"]),
		InstallHint:        strings.TrimSpace(entry.Metadata["install_hint"]),
		FullAccessModeID:   strings.TrimSpace(entry.Metadata["full_access_mode_id"]),
		EnvVars:            parseOverlayEnv(entry.Metadata),
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
