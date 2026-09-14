package pluginsource

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/registry/gitsrc"
)

const MaxDocumentBytes = 2 << 20

var (
	ErrDocumentTooLarge = errors.New("marketplace_document_too_large")
	ErrNotMarketplace   = errors.New("marketplace_not_a_marketplace")
	pluginNamePattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
)

type Diagnostic struct {
	Plugin  string `json:"plugin,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Document struct {
	Name, Owner, Path string
	SourceRef         string
	ResolvedRef       string
	DigestSHA256      string
	Plugins           []Plugin
	Diagnostics       []Diagnostic
}

type Plugin struct {
	Name, Description, Version, Author, Homepage, Repository, License, Category, Icon string
	Keywords                                                                          []string
	Source                                                                            PluginSource
}

type Kind string

const (
	SourceRelative Kind = "relative"
	SourceGitHub   Kind = "github"
	SourceHTTPS    Kind = "https"
)

type PluginSource struct {
	Kind   Kind
	Ref    string
	Path   string
	GitRef string
}

// DecodeDocument tolerates optional metadata drift while validating each plugin's identity and source.
func DecodeDocument(raw []byte) (Document, error) {
	if len(raw) > MaxDocumentBytes {
		return Document{}, ErrDocumentTooLarge
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return Document{}, fmt.Errorf("%w: invalid JSON: %w", ErrNotMarketplace, err)
	}
	var plugins []json.RawMessage
	if err := json.Unmarshal(envelope["plugins"], &plugins); err != nil || plugins == nil {
		return Document{}, fmt.Errorf("%w: plugins must be an array", ErrNotMarketplace)
	}
	digest := sha256.Sum256(raw)
	document := Document{
		Name: optionalString(envelope["name"]), Owner: authorName(envelope["owner"]),
		DigestSHA256: hex.EncodeToString(digest[:]), Plugins: make([]Plugin, 0, len(plugins)),
	}
	seen := make(map[string]bool, len(plugins))
	for index, item := range plugins {
		plugin, diagnostic := decodePlugin(item)
		if diagnostic == nil && seen[plugin.Name] {
			diagnostic = &Diagnostic{
				Plugin:  plugin.Name,
				Code:    diagnosticcontract.CodeMarketplacePluginDuplicate,
				Message: "plugin name is duplicated",
			}
		}
		if diagnostic != nil {
			diagnostic.Message = fmt.Sprintf("plugins[%d]: %s", index, diagnostic.Message)
			document.Diagnostics = append(document.Diagnostics, *diagnostic)
			continue
		}
		seen[plugin.Name] = true
		document.Plugins = append(document.Plugins, plugin)
	}
	return document, nil
}

func decodePlugin(raw json.RawMessage) (Plugin, *Diagnostic) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return Plugin{}, &Diagnostic{
			Code:    diagnosticcontract.CodeMarketplacePluginInvalid,
			Message: "plugin must be an object",
		}
	}
	name := optionalString(fields["name"])
	if !pluginNamePattern.MatchString(name) {
		return Plugin{}, &Diagnostic{
			Code:    diagnosticcontract.CodeMarketplacePluginInvalid,
			Message: "plugin name is invalid",
		}
	}
	source, err := decodePluginSource(fields["source"])
	if err != nil {
		return Plugin{}, &Diagnostic{
			Plugin:  name,
			Code:    diagnosticcontract.CodeMarketplacePluginSourceUnsupported,
			Message: err.Error(),
		}
	}
	return Plugin{
		Name: name, Source: source,
		Description: optionalString(fields["description"]), Version: optionalString(fields["version"]),
		Author: authorName(fields["author"]), Homepage: optionalString(fields["homepage"]),
		Repository: optionalString(fields["repository"]), License: optionalString(fields["license"]),
		Category: optionalString(fields["category"]), Icon: optionalString(fields["icon"]),
		Keywords: optionalStrings(fields["keywords"]),
	}, nil
}

func decodePluginSource(raw json.RawMessage) (PluginSource, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err == nil && value != "" {
		return decodeStringSource(value)
	}
	var source struct {
		Source string `json:"source"`
		Repo   string `json:"repo"`
		Ref    string `json:"ref"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal(raw, &source); err != nil || source.Source != "github" {
		return PluginSource{}, errors.New("source must be a relative path, GitHub object, or HTTPS URL")
	}
	reference, err := normalizeGitHubRepository(source.Repo)
	if err != nil {
		return PluginSource{}, errors.New("GitHub source requires an owner/repository")
	}
	return PluginSource{Kind: SourceGitHub, Ref: reference, GitRef: source.Ref, Path: source.Path}, nil
}

func decodeStringSource(value string) (PluginSource, error) {
	if strings.HasPrefix(value, "https://") {
		if err := gitsrc.ValidateRepositoryRef(value); err != nil {
			return PluginSource{}, errors.New(
				"HTTPS source must be public and contain no credentials, query or fragment",
			)
		}
		return PluginSource{Kind: SourceHTTPS, Ref: value}, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || strings.HasPrefix(value, "/") ||
		strings.ContainsAny(value, "\\\x00") || strings.TrimSpace(value) == "" {
		return PluginSource{}, errors.New("source path must be relative to the marketplace checkout")
	}
	return PluginSource{Kind: SourceRelative, Path: value}, nil
}

func optionalString(raw json.RawMessage) string {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

func optionalStrings(raw json.RawMessage) []string {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	return values
}

func authorName(raw json.RawMessage) string {
	if value := optionalString(raw); value != "" {
		return value
	}
	var author struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &author); err != nil {
		return ""
	}
	return author.Name
}
