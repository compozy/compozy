package agentplugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/compozy/compozy/internal/fileutil"
)

var manifestFields = map[string]struct{}{
	fieldSchema: {}, fieldName: {}, fieldVersion: {}, fieldDescription: {}, fieldAuthor: {},
	fieldHomepage: {}, fieldRepository: {}, fieldLicense: {}, fieldKeywords: {}, fieldExtensions: {},
}

const (
	manifestFileName       = "plugin.json"
	jsonObjectIssueMessage = "must be a JSON object"
)

// Load validates the root manifest and independently discovers portable
// components. Only root-manifest failures are returned as errors.
func Load(dir string, opts LoadOptions) (*Package, error) {
	root, err := canonicalExistingPrefix(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve Agent Plugins root %q: %w", dir, err)
	}
	content, err := readManifestContent(root)
	if err != nil {
		return nil, err
	}

	pkg, schema, err := decodeManifest(content)
	if err != nil {
		return nil, err
	}
	discoverSkills(root, pkg)
	loadMCP(root, opts.DataDir, schema, pkg)
	sort.Slice(pkg.Diagnostics, func(left, right int) bool {
		if pkg.Diagnostics[left].Scope == pkg.Diagnostics[right].Scope {
			return pkg.Diagnostics[left].Message < pkg.Diagnostics[right].Message
		}
		return pkg.Diagnostics[left].Scope < pkg.Diagnostics[right].Scope
	})
	return pkg, nil
}

// ReadManifestName reads only the authored package name needed to resolve its data directory.
// Load still performs the complete schema and component validation exactly once afterward.
func ReadManifestName(dir string) (string, error) {
	root, err := canonicalExistingPrefix(dir)
	if err != nil {
		return "", fmt.Errorf("resolve Agent Plugins root %q: %w", dir, err)
	}
	content, err := readManifestContent(root)
	if err != nil {
		return "", err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(content, &fields); err != nil || fields == nil {
		return "", &ManifestError{Issues: []Issue{{Path: "$", Message: jsonObjectIssueMessage}}}
	}
	issues := make([]Issue, 0, 1)
	name, ok := requiredString(fields, fieldName, &issues)
	if ok {
		if err := ValidateName(name); err != nil {
			issues = append(issues, Issue{Path: fieldName, Message: fmt.Sprintf("name %q %s", name, err)})
		}
	}
	if len(issues) > 0 {
		return "", &ManifestError{Issues: issues}
	}
	return name, nil
}

func readManifestContent(root string) ([]byte, error) {
	manifestPath := filepath.Join(root, manifestFileName)
	content, _, err := fileutil.ReadRegularFile(manifestPath)
	if err == nil {
		return content, nil
	}
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, &ManifestError{Issues: []Issue{{Path: "$", Message: "plugin.json is required"}}}
	case errors.Is(err, fileutil.ErrSymlink),
		errors.Is(err, fileutil.ErrDirectory),
		errors.Is(err, fileutil.ErrNotRegular):
		return nil, &ManifestError{Issues: []Issue{{Path: "$", Message: "plugin.json must be a regular file"}}}
	default:
		return nil, fmt.Errorf("read Agent Plugins manifest %q: %w", manifestPath, err)
	}
}

func decodeManifest(content []byte) (*Package, string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(content, &fields); err != nil {
		return nil, "", &ManifestError{Issues: []Issue{{Path: "$", Message: jsonObjectIssueMessage}}}
	}
	if fields == nil {
		return nil, "", &ManifestError{Issues: []Issue{{Path: "$", Message: jsonObjectIssueMessage}}}
	}

	pkg := &Package{}
	issues := make([]Issue, 0)
	schema, ok := requiredString(fields, fieldSchema, &issues)
	if ok && schema != PluginSchemaID {
		issues = append(issues, Issue{Path: fieldSchema, Message: "must equal " + PluginSchemaID})
	}
	name, nameOK := requiredString(fields, fieldName, &issues)
	if nameOK {
		if err := ValidateName(name); err != nil {
			issues = append(issues, Issue{Path: fieldName, Message: fmt.Sprintf("name %q %s", name, err)})
		}
	}
	pkg.Name = name
	pkg.Version = optionalString(fields, fieldVersion, &issues)
	pkg.Description = optionalString(fields, fieldDescription, &issues)
	pkg.Homepage = optionalString(fields, fieldHomepage, &issues)
	pkg.Repository = optionalString(fields, fieldRepository, &issues)
	pkg.License = optionalString(fields, fieldLicense, &issues)
	pkg.Keywords = optionalStrings(fields, fieldKeywords, &issues)
	pkg.Author = optionalAuthor(fields, &issues)
	validateExtensions(fields, pkg, &issues)

	unknown := make([]string, 0)
	for field := range fields {
		if _, known := manifestFields[field]; !known {
			unknown = append(unknown, field)
		}
	}
	sort.Strings(unknown)
	for _, field := range unknown {
		pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{
			Scope: scopeManifest, Message: "ignored unknown top-level field: " + field,
		})
	}
	if len(issues) > 0 {
		return nil, "", &ManifestError{Issues: issues}
	}
	return pkg, schema, nil
}

func requiredString(fields map[string]json.RawMessage, name string, issues *[]Issue) (string, bool) {
	raw, exists := fields[name]
	if !exists {
		*issues = append(*issues, Issue{Path: name, Message: "is required"})
		return "", false
	}
	var value string
	if isJSONNull(raw) || json.Unmarshal(raw, &value) != nil {
		*issues = append(*issues, Issue{Path: name, Message: "must be a string"})
		return "", false
	}
	return value, true
}

func optionalString(fields map[string]json.RawMessage, name string, issues *[]Issue) string {
	return optionalStringAt(fields, name, name, issues)
}

func optionalStringAt(fields map[string]json.RawMessage, name string, path string, issues *[]Issue) string {
	raw, exists := fields[name]
	if !exists {
		return ""
	}
	var value string
	if isJSONNull(raw) || json.Unmarshal(raw, &value) != nil {
		*issues = append(*issues, Issue{Path: path, Message: "must be a string when present"})
	}
	return value
}

func optionalStrings(fields map[string]json.RawMessage, name string, issues *[]Issue) []string {
	raw, exists := fields[name]
	if !exists {
		return nil
	}
	values, ok := decodeStringSlice(raw)
	if !ok {
		*issues = append(*issues, Issue{Path: name, Message: "must be an array of strings when present"})
		return nil
	}
	return values
}

func decodeStringSlice(raw json.RawMessage) ([]string, bool) {
	if isJSONNull(raw) {
		return nil, false
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil || entries == nil {
		return nil, false
	}
	values := make([]string, len(entries))
	for idx, entry := range entries {
		if isJSONNull(entry) || json.Unmarshal(entry, &values[idx]) != nil {
			return nil, false
		}
	}
	return values, true
}

func optionalAuthor(fields map[string]json.RawMessage, issues *[]Issue) *AuthorInfo {
	raw, exists := fields[fieldAuthor]
	if !exists {
		return nil
	}
	if isJSONNull(raw) {
		*issues = append(*issues, Issue{Path: fieldAuthor, Message: messageObjectWhenPresent})
		return nil
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		*issues = append(*issues, Issue{Path: fieldAuthor, Message: messageObjectWhenPresent})
		return nil
	}
	allowed := map[string]struct{}{fieldName: {}, fieldEmail: {}, fieldURL: {}}
	unknown := make([]string, 0)
	for field := range values {
		if _, known := allowed[field]; !known {
			unknown = append(unknown, field)
		}
	}
	sort.Strings(unknown)
	for _, field := range unknown {
		*issues = append(*issues, Issue{Path: fieldAuthor + "." + field, Message: "unknown field"})
	}
	return &AuthorInfo{
		Name:  optionalStringAt(values, fieldName, fieldAuthor+"."+fieldName, issues),
		Email: optionalStringAt(values, fieldEmail, fieldAuthor+"."+fieldEmail, issues),
		URL:   optionalStringAt(values, fieldURL, fieldAuthor+"."+fieldURL, issues),
	}
}

func validateExtensions(fields map[string]json.RawMessage, pkg *Package, issues *[]Issue) {
	raw, exists := fields[fieldExtensions]
	if !exists {
		return
	}
	if isJSONNull(raw) {
		*issues = append(*issues, Issue{Path: fieldExtensions, Message: messageObjectWhenPresent})
		return
	}
	var namespaces map[string]json.RawMessage
	if err := json.Unmarshal(raw, &namespaces); err != nil || namespaces == nil {
		pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{
			Scope: scopeManifest, Message: "ignored non-object extensions field",
		})
	}
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
