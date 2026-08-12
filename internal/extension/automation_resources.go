package extensionpkg

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	automationpkg "github.com/compozy/compozy/internal/automation"
)

// ExtensionJob is one package-authored scheduled automation definition.
type ExtensionJob struct {
	Name      string                        `toml:"name"`
	Agent     string                        `toml:"agent"`
	Prompt    string                        `toml:"prompt"`
	Schedule  automationpkg.ScheduleSpec    `toml:"schedule"`
	Task      *automationpkg.JobTaskConfig  `toml:"task,omitempty"`
	Enabled   *bool                         `toml:"enabled,omitempty"`
	Retry     automationpkg.RetryConfig     `toml:"retry,omitempty"`
	FireLimit automationpkg.FireLimitConfig `toml:"fire_limit,omitempty"`
}

// ExtensionTrigger is one package-authored event automation definition.
type ExtensionTrigger struct {
	Name         string                        `toml:"name"`
	Agent        string                        `toml:"agent"`
	Prompt       string                        `toml:"prompt"`
	Event        string                        `toml:"event"`
	Filter       map[string]string             `toml:"filter,omitempty"`
	Enabled      *bool                         `toml:"enabled,omitempty"`
	Retry        automationpkg.RetryConfig     `toml:"retry,omitempty"`
	FireLimit    automationpkg.FireLimitConfig `toml:"fire_limit,omitempty"`
	EndpointSlug string                        `toml:"endpoint_slug,omitempty"`
}

type automationResourceDocument struct {
	Jobs     []ExtensionJob     `toml:"jobs"`
	Triggers []ExtensionTrigger `toml:"triggers"`
}

// LoadAutomationResources loads and validates extension automation TOML resources.
func LoadAutomationResources(
	rootDir string,
	extensionName string,
	paths []string,
) ([]automationpkg.Job, []automationpkg.Trigger, error) {
	files, err := collectDeclaredResourceFiles(rootDir, paths, ".toml", "automation resource")
	if err != nil {
		return nil, nil, err
	}
	jobsByName := make(map[string]automationpkg.Job)
	triggersByName := make(map[string]automationpkg.Trigger)
	jobOrigins := make(map[string]string)
	triggerOrigins := make(map[string]string)
	for _, file := range files {
		document, err := decodeAutomationResourceDocument(file)
		if err != nil {
			return nil, nil, err
		}
		for _, job := range document.Jobs {
			normalized := normalizeExtensionJob(job)
			if _, exists := jobsByName[normalized.Name]; exists {
				return nil, nil, wrapResourceValidationError(file, fmt.Errorf(
					"%w: duplicate automation job %q also declared in %q",
					ErrManifestInvalid,
					normalized.Name,
					jobOrigins[normalized.Name],
				))
			}
			projected := projectExtensionJob(extensionName, normalized)
			if err := projected.Validate("automation.jobs"); err != nil {
				return nil, nil, wrapResourceValidationError(
					file,
					fmt.Errorf("%w: automation resource: %w", ErrManifestInvalid, err),
				)
			}
			jobsByName[normalized.Name] = projected
			jobOrigins[normalized.Name] = file
		}
		for _, trigger := range document.Triggers {
			normalized, err := normalizeExtensionTrigger(trigger)
			if err != nil {
				return nil, nil, wrapResourceValidationError(
					file,
					fmt.Errorf("%w: automation resource: %w", ErrManifestInvalid, err),
				)
			}
			if _, exists := triggersByName[normalized.Name]; exists {
				return nil, nil, wrapResourceValidationError(file, fmt.Errorf(
					"%w: duplicate automation trigger %q also declared in %q",
					ErrManifestInvalid,
					normalized.Name,
					triggerOrigins[normalized.Name],
				))
			}
			projected := projectExtensionTrigger(extensionName, normalized)
			if err := projected.Validate("automation.triggers"); err != nil {
				return nil, nil, wrapResourceValidationError(
					file,
					fmt.Errorf("%w: automation resource: %w", ErrManifestInvalid, err),
				)
			}
			triggersByName[normalized.Name] = projected
			triggerOrigins[normalized.Name] = file
		}
	}
	jobs := make([]automationpkg.Job, 0, len(jobsByName))
	for _, name := range sortedKeys(jobsByName) {
		jobs = append(jobs, jobsByName[name])
	}
	triggers := make([]automationpkg.Trigger, 0, len(triggersByName))
	for _, name := range sortedKeys(triggersByName) {
		triggers = append(triggers, triggersByName[name])
	}
	return jobs, triggers, nil
}

func decodeAutomationResourceDocument(file string) (automationResourceDocument, error) {
	body, err := os.ReadFile(file)
	if err != nil {
		return automationResourceDocument{}, fmt.Errorf("extension: read automation resource %q: %w", file, err)
	}
	var document automationResourceDocument
	metadata, err := toml.Decode(string(body), &document)
	if err != nil {
		return automationResourceDocument{}, wrapResourceValidationError(
			file,
			fmt.Errorf("%w: decode automation resource: %w", ErrManifestInvalid, err),
		)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
		return automationResourceDocument{}, wrapResourceValidationError(
			file,
			fmt.Errorf("%w: automation resource has unknown field %q", ErrManifestInvalid, undecoded[0].String()),
		)
	}
	return document, nil
}

func normalizeExtensionJob(value ExtensionJob) ExtensionJob {
	value.Name = strings.TrimSpace(value.Name)
	value.Agent = strings.TrimSpace(value.Agent)
	value.Prompt = strings.TrimSpace(value.Prompt)
	enabled := true
	if value.Enabled != nil {
		enabled = *value.Enabled
	}
	value.Enabled = &enabled
	if value.Retry.Strategy == "" {
		value.Retry = automationpkg.DefaultRetryConfig()
	}
	if value.FireLimit.Max == 0 || strings.TrimSpace(value.FireLimit.Window) == "" {
		value.FireLimit = automationpkg.DefaultFireLimitConfig()
	}
	return value
}

func normalizeExtensionTrigger(value ExtensionTrigger) (ExtensionTrigger, error) {
	value.Name = strings.TrimSpace(value.Name)
	value.Agent = strings.TrimSpace(value.Agent)
	value.Prompt = strings.TrimSpace(value.Prompt)
	value.EndpointSlug = strings.TrimSpace(value.EndpointSlug)
	if value.Event == "webhook" {
		return ExtensionTrigger{}, fmt.Errorf(
			"automation.triggers.event %q is not supported for package resources",
			value.Event,
		)
	}
	enabled := true
	if value.Enabled != nil {
		enabled = *value.Enabled
	}
	value.Enabled = &enabled
	if value.Retry.Strategy == "" {
		value.Retry = automationpkg.DefaultRetryConfig()
	}
	if value.FireLimit.Max == 0 || strings.TrimSpace(value.FireLimit.Window) == "" {
		value.FireLimit = automationpkg.DefaultFireLimitConfig()
	}
	return value, nil
}

func projectExtensionJob(extensionName string, value ExtensionJob) automationpkg.Job {
	namespace := strings.TrimSpace(extensionName)
	name := namespace + "/" + value.Name
	return automationpkg.Job{
		ID:        "extension/" + namespace + "/automation.job/" + value.Name,
		Scope:     automationpkg.AutomationScopeGlobal,
		Name:      name,
		AgentName: value.Agent,
		Prompt:    value.Prompt,
		Schedule:  new(value.Schedule),
		Task:      cloneAutomationTaskConfig(value.Task),
		Enabled:   *value.Enabled,
		Retry:     value.Retry,
		FireLimit: value.FireLimit,
		Source:    automationpkg.JobSourcePackage,
	}
}

func projectExtensionTrigger(extensionName string, value ExtensionTrigger) automationpkg.Trigger {
	namespace := strings.TrimSpace(extensionName)
	name := namespace + "/" + value.Name
	return automationpkg.Trigger{
		ID:           "extension/" + namespace + "/automation.trigger/" + value.Name,
		Scope:        automationpkg.AutomationScopeGlobal,
		Name:         name,
		AgentName:    value.Agent,
		Prompt:       value.Prompt,
		Event:        value.Event,
		Filter:       cloneStringMap(value.Filter),
		Enabled:      *value.Enabled,
		Retry:        value.Retry,
		FireLimit:    value.FireLimit,
		Source:       automationpkg.JobSourcePackage,
		EndpointSlug: value.EndpointSlug,
	}
}

func validateAutomationAgentTargets(
	jobs []automationpkg.Job,
	triggers []automationpkg.Trigger,
	agents []StaticAgent,
) error {
	available := make(map[string]struct{}, len(agents))
	for _, agent := range agents {
		available[strings.TrimSpace(agent.Agent.Name)] = struct{}{}
	}
	shipped := sortedKeys(available)
	for _, job := range jobs {
		if _, ok := available[job.AgentName]; !ok {
			return fmt.Errorf(
				"%w: automation job %q targets unshipped agent %q; shipped agents: %v",
				ErrManifestInvalid,
				job.Name,
				job.AgentName,
				shipped,
			)
		}
	}
	for _, trigger := range triggers {
		if _, ok := available[trigger.AgentName]; !ok {
			return fmt.Errorf(
				"%w: automation trigger %q targets unshipped agent %q; shipped agents: %v",
				ErrManifestInvalid,
				trigger.Name,
				trigger.AgentName,
				shipped,
			)
		}
	}
	return nil
}

func collectDeclaredResourceFiles(rootDir string, paths []string, extension, label string) ([]string, error) {
	files := make([]string, 0)
	seen := make(map[string]struct{})
	for _, declared := range paths {
		resolved, err := resolveResourcePathWithinRoot(rootDir, declared, label)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, fmt.Errorf("extension: stat %s %q: %w", label, declared, err)
		}
		if info.Mode().IsRegular() {
			if !strings.EqualFold(filepath.Ext(resolved), extension) {
				return nil, fmt.Errorf("%w: %s %q must be a %s file", ErrManifestInvalid, label, declared, extension)
			}
			seen[resolved] = struct{}{}
			continue
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("%w: %s %q must be a regular file or directory", ErrManifestInvalid, label, declared)
		}
		if strings.EqualFold(filepath.Ext(resolved), extension) {
			return nil, fmt.Errorf(
				"%w: %s %q must be a regular %s file",
				ErrManifestInvalid,
				label,
				declared,
				extension,
			)
		}
		err = filepath.WalkDir(resolved, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), extension) {
				return nil
			}
			relative, err := relativeResourcePath(rootDir, path)
			if err != nil {
				return err
			}
			securePath, err := resolveResourcePathWithinRoot(rootDir, relative, label)
			if err != nil {
				return err
			}
			seen[securePath] = struct{}{}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("extension: walk %s %q: %w", label, declared, err)
		}
	}
	for file := range seen {
		files = append(files, file)
	}
	slices.Sort(files)
	return files, nil
}
