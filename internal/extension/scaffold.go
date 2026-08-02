package extensionpkg

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

//go:embed scaffold_templates/**
var extensionScaffoldTemplates embed.FS

const (
	scaffoldTemplateRoot = "scaffold_templates"
	scaffoldGoSDKVersion = "v0.3.0-beta.3"
)

var extensionNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ScaffoldTemplate identifies one embedded extension authoring template.
type ScaffoldTemplate string

const (
	ScaffoldTemplateToolProviderTS  ScaffoldTemplate = "tool-provider-ts"
	ScaffoldTemplateToolProviderGo  ScaffoldTemplate = "tool-provider-go"
	ScaffoldTemplateHookTS          ScaffoldTemplate = "hook-ts"
	ScaffoldTemplateMemoryBackendTS ScaffoldTemplate = "memory-backend-ts"
	// #nosec G101 -- this public scaffold identifier is not a credential.
	ScaffoldTemplateLoopWatchSourceGo ScaffoldTemplate = "loop-watch-source-go"
)

var scaffoldTemplates = []ScaffoldTemplate{
	ScaffoldTemplateHookTS,
	ScaffoldTemplateLoopWatchSourceGo,
	ScaffoldTemplateMemoryBackendTS,
	ScaffoldTemplateToolProviderGo,
	ScaffoldTemplateToolProviderTS,
}

// ScaffoldRequest configures one extension project created from embedded assets.
type ScaffoldRequest struct {
	Name      string
	Template  ScaffoldTemplate
	Directory string
}

// ScaffoldResult describes the created project.
type ScaffoldResult struct {
	Name      string           `json:"name"`
	Template  ScaffoldTemplate `json:"template"`
	Directory string           `json:"directory"`
	Files     []string         `json:"files"`
}

// ScaffoldTemplates returns every supported template in stable order.
func ScaffoldTemplates() []ScaffoldTemplate {
	return slices.Clone(scaffoldTemplates)
}

// ScaffoldExtension creates a project atomically from the binary's embedded templates.
func ScaffoldExtension(req ScaffoldRequest) (result *ScaffoldResult, resultErr error) {
	name := normalizeExtensionScaffoldName(req.Name)
	if !extensionNamePattern.MatchString(name) {
		return nil, fmt.Errorf("extension: invalid extension name %q", req.Name)
	}
	template := req.Template
	if template == "" {
		template = ScaffoldTemplateToolProviderTS
	}
	if !slices.Contains(scaffoldTemplates, template) {
		return nil, fmt.Errorf("extension: unknown scaffold template %q", template)
	}
	target, err := scaffoldTargetDirectory(req.Directory, name)
	if err != nil {
		return nil, err
	}
	if err := requireEmptyScaffoldTarget(target); err != nil {
		return nil, err
	}

	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, fmt.Errorf("extension: create scaffold parent %q: %w", parent, err)
	}
	stage, err := os.MkdirTemp(parent, ".compozy-extension-")
	if err != nil {
		return nil, fmt.Errorf("extension: create scaffold staging directory: %w", err)
	}
	cleanupStage := true
	defer func() {
		if cleanupStage {
			if cleanupErr := os.RemoveAll(stage); cleanupErr != nil {
				result = nil
				resultErr = errors.Join(
					resultErr,
					fmt.Errorf("extension: clean scaffold staging directory: %w", cleanupErr),
				)
			}
		}
	}()

	files, err := renderScaffoldTemplate(stage, name, template)
	if err != nil {
		return nil, err
	}
	if err := replaceEmptyScaffoldTarget(target); err != nil {
		return nil, err
	}
	if err := os.Rename(stage, target); err != nil {
		return nil, fmt.Errorf("extension: publish scaffold %q: %w", target, err)
	}
	cleanupStage = false
	return &ScaffoldResult{Name: name, Template: template, Directory: target, Files: files}, nil
}

func normalizeExtensionScaffoldName(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	var result strings.Builder
	lastDash := false
	for _, char := range trimmed {
		valid := char >= 'a' && char <= 'z' || char >= '0' && char <= '9'
		if valid {
			result.WriteRune(char)
			lastDash = false
			continue
		}
		if result.Len() > 0 && !lastDash {
			result.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(result.String(), "-")
}

func scaffoldTargetDirectory(directory, name string) (string, error) {
	target := strings.TrimSpace(directory)
	if target == "" {
		target = name
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("extension: resolve scaffold target %q: %w", target, err)
	}
	return absTarget, nil
}

func requireEmptyScaffoldTarget(target string) error {
	entries, err := os.ReadDir(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("extension: read scaffold target %q: %w", target, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("extension: target directory is not empty: %s", target)
	}
	return nil
}

func replaceEmptyScaffoldTarget(target string) error {
	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("extension: stat scaffold target %q: %w", target, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("extension: scaffold target %q is not a directory", target)
	}
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("extension: replace empty scaffold target %q: %w", target, err)
	}
	return nil
}

func renderScaffoldTemplate(stage, name string, template ScaffoldTemplate) ([]string, error) {
	root := filepath.ToSlash(filepath.Join(scaffoldTemplateRoot, string(template)))
	files := make([]string, 0)
	err := fs.WalkDir(extensionScaffoldTemplates, root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = strings.TrimSuffix(relative, ".template")
		content, err := extensionScaffoldTemplates.ReadFile(path)
		if err != nil {
			return err
		}
		rendered := strings.NewReplacer(
			"__EXTENSION_NAME__", name,
			"__COMPOZY_GO_SDK_VERSION__", scaffoldGoSDKVersion,
		).Replace(string(content))
		destination := filepath.Join(stage, relative)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(destination, []byte(rendered), 0o600); err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("extension: render scaffold template %q: %w", template, err)
	}
	slices.Sort(files)
	return files, nil
}
