package knowledge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

type sourceKind int

const (
	sourceKindUnknown sourceKind = iota
	sourceKindFile
	sourceKindDirectory
	sourceKindURL
	sourceKindAPI
)

var (
	supportedAPISourceProviders = map[string]struct{}{
		"confluence": {},
		"zendesk":    {},
		"notion":     {},
		"salesforce": {},
		"servicenow": {},
		"github":     {},
		"intercom":   {},
		"freshdesk":  {},
		"linear":     {},
	}
	apiProviderList = sortedKeys(supportedAPISourceProviders)
)

// SourceBuilder constructs knowledge source configurations for knowledge bases.
type SourceBuilder struct {
	config *engineknowledge.SourceConfig
	errors []error
	kind   sourceKind
}

// NewFileSource creates a knowledge source referencing a single local file.
func NewFileSource(path string) *SourceBuilder {
	normalized := normalizePath(path)
	return &SourceBuilder{
		config: &engineknowledge.SourceConfig{
			Type: engineknowledge.SourceTypeMarkdownGlob,
			Path: normalized,
		},
		errors: make([]error, 0),
		kind:   sourceKindFile,
	}
}

// NewDirectorySource creates a knowledge source that ingests content from one or more directories.
func NewDirectorySource(paths ...string) *SourceBuilder {
	sanitized := normalizePaths(paths)
	primary, additional := splitPrimaryAndAdditional(sanitized)
	return &SourceBuilder{
		config: &engineknowledge.SourceConfig{
			Type:  engineknowledge.SourceTypeMarkdownGlob,
			Path:  primary,
			Paths: additional,
		},
		errors: make([]error, 0),
		kind:   sourceKindDirectory,
	}
}

// NewURLSource creates a knowledge source that fetches content from one or more remote URLs.
func NewURLSource(urls ...string) *SourceBuilder {
	sanitized := normalizeURLStrings(urls)
	primary, additional := splitPrimaryAndAdditional(sanitized)
	return &SourceBuilder{
		config: &engineknowledge.SourceConfig{
			Type: engineknowledge.SourceTypeURL,
			Path: primary,
			URLs: additional,
		},
		errors: make([]error, 0),
		kind:   sourceKindURL,
	}
}

// NewAPISource creates a knowledge source that ingests content from a supported API provider.
func NewAPISource(provider string) *SourceBuilder {
	normalized := strings.ToLower(strings.TrimSpace(provider))
	return &SourceBuilder{
		config: &engineknowledge.SourceConfig{
			Type:     engineknowledge.SourceTypeURL,
			Provider: normalized,
		},
		errors: make([]error, 0),
		kind:   sourceKindAPI,
	}
}

// Build validates the accumulated configuration and returns a cloned source config.
func (b *SourceBuilder) Build(ctx context.Context) (*engineknowledge.SourceConfig, error) {
	if b == nil {
		return nil, fmt.Errorf("source builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}

	log := logger.FromContext(ctx)
	log.Debug("building knowledge source configuration", "type", b.config.Type, "provider", b.config.Provider)

	collected := make([]error, 0, len(b.errors)+6)
	collected = append(collected, b.errors...)
	collected = append(collected, b.validateKind())
	switch b.kind {
	case sourceKindFile:
		collected = append(collected, b.validateFile(ctx)...)
	case sourceKindDirectory:
		collected = append(collected, b.validateDirectory(ctx)...)
	case sourceKindURL:
		collected = append(collected, b.validateURL(ctx)...)
	case sourceKindAPI:
		collected = append(collected, b.validateAPI(ctx)...)
	}

	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}

	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone source config: %w", err)
	}
	return cloned, nil
}

func (b *SourceBuilder) validateKind() error {
	if b.kind == sourceKindUnknown {
		return fmt.Errorf("source type is not specified")
	}
	return nil
}

func (b *SourceBuilder) validateFile(ctx context.Context) []error {
	path := normalizePath(b.config.Path)
	b.config.Path = path
	if err := validate.NonEmpty(ctx, "path", path); err != nil {
		return []error{err}
	}
	info, err := os.Stat(path)
	if err != nil {
		return []error{fmt.Errorf("file source path %q is not accessible: %w", path, err)}
	}
	if info.IsDir() {
		return []error{fmt.Errorf("file source path %q must be a file", path)}
	}
	handle, openErr := os.Open(path)
	if openErr != nil {
		return []error{fmt.Errorf("file source path %q cannot be opened: %w", path, openErr)}
	}
	handle.Close()
	b.config.Paths = nil
	b.config.URLs = nil
	return nil
}

func (b *SourceBuilder) validateDirectory(ctx context.Context) []error {
	input := append([]string{b.config.Path}, b.config.Paths...)
	paths := normalizePaths(input)
	if err := validate.Required(ctx, "paths", paths); err != nil {
		return []error{err}
	}
	errs := make([]error, 0, len(paths))
	for _, dir := range paths {
		info, err := os.Stat(dir)
		if err != nil {
			errs = append(errs, fmt.Errorf("directory source path %q is not accessible: %w", dir, err))
			continue
		}
		if !info.IsDir() {
			errs = append(errs, fmt.Errorf("directory source path %q must be a directory", dir))
		}
	}
	if len(errs) > 0 {
		return errs
	}
	primary, additional := splitPrimaryAndAdditional(paths)
	b.config.Path = primary
	if len(additional) > 0 {
		b.config.Paths = append(make([]string, 0, len(additional)), additional...)
	} else {
		b.config.Paths = nil
	}
	b.config.URLs = nil
	return nil
}

func (b *SourceBuilder) validateURL(ctx context.Context) []error {
	input := append([]string{b.config.Path}, b.config.URLs...)
	urls := normalizeURLStrings(input)
	if err := validate.Required(ctx, "urls", urls); err != nil {
		return []error{err}
	}
	errs := make([]error, 0, len(urls))
	for _, raw := range urls {
		if err := validate.URL(ctx, raw); err != nil {
			errs = append(errs, fmt.Errorf("url source %q is invalid: %w", raw, err))
		}
	}
	if len(errs) > 0 {
		return errs
	}
	primary, additional := splitPrimaryAndAdditional(urls)
	b.config.Path = primary
	if len(additional) > 0 {
		b.config.URLs = append(make([]string, 0, len(additional)), additional...)
	} else {
		b.config.URLs = nil
	}
	b.config.Paths = nil
	return nil
}

func (b *SourceBuilder) validateAPI(ctx context.Context) []error {
	provider := strings.ToLower(strings.TrimSpace(b.config.Provider))
	if err := validate.NonEmpty(ctx, "provider", provider); err != nil {
		return []error{err}
	}
	if _, ok := supportedAPISourceProviders[provider]; !ok {
		return []error{fmt.Errorf(
			"api source provider %q is not supported; must be one of %s",
			provider,
			strings.Join(apiProviderList, ", "),
		)}
	}
	b.config.Provider = provider
	b.config.Path = ""
	b.config.Paths = nil
	b.config.URLs = nil
	return nil
}

func normalizePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(trimmed)
}

func normalizePaths(paths []string) []string {
	normalized := make([]string, 0, len(paths))
	seen := make(map[string]struct{})
	for _, raw := range paths {
		cleaned := normalizePath(raw)
		if cleaned == "" {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		normalized = append(normalized, cleaned)
	}
	return normalized
}

func normalizeURLStrings(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func splitPrimaryAndAdditional(values []string) (string, []string) {
	if len(values) == 0 {
		return "", nil
	}
	if len(values) == 1 {
		return values[0], nil
	}
	additional := make([]string, 0, len(values)-1)
	additional = append(additional, values[1:]...)
	return values[0], additional
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
