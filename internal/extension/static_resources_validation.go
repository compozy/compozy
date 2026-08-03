package extensionpkg

import (
	"context"
	"fmt"
)

type staticResourcePathGroup struct {
	label string
	paths []string
}

func staticResourcePathGroups(resources ResourcesConfig) []staticResourcePathGroup {
	return []staticResourcePathGroup{
		{label: "skill resource", paths: resources.Skills},
		{label: "loop resource", paths: resources.Loops},
		{label: "agent resource", paths: resources.Agents},
		{label: "automation resource", paths: resources.Automation},
		{label: "layout resource", paths: resources.Layouts},
	}
}

func validateStaticKitResources(ctx context.Context, rootDir string, manifest *Manifest) error {
	if manifest == nil {
		return nil
	}
	if err := validateStaticResourcePaths(rootDir, manifest.Resources); err != nil {
		return err
	}
	agents, err := LoadAgentResources(rootDir, manifest.Resources.Agents)
	if err != nil {
		return err
	}
	jobs, triggers, err := LoadAutomationResources(rootDir, manifest.Name, manifest.Resources.Automation)
	if err != nil {
		return err
	}
	if err := validateAutomationAgentTargets(jobs, triggers, agents); err != nil {
		return err
	}
	if _, err := LoadLayoutResources(ctx, rootDir, manifest.Resources.Layouts); err != nil {
		return err
	}
	return nil
}

func validateStaticResourcePaths(rootDir string, resources ResourcesConfig) error {
	for _, group := range staticResourcePathGroups(resources) {
		for _, path := range group.paths {
			if _, err := resolveResourcePathWithinRoot(rootDir, path, group.label); err != nil {
				return fmt.Errorf("extension: validate declared %s: %w", group.label, err)
			}
		}
	}
	return nil
}
