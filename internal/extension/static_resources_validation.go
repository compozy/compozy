package extensionpkg

import "fmt"

func validateStaticKitResources(rootDir string, manifest *Manifest) error {
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
	jobs, triggers, err := LoadAutomationResources(rootDir, manifest.Resources.Automation)
	if err != nil {
		return err
	}
	if err := validateAutomationAgentTargets(jobs, triggers, agents); err != nil {
		return err
	}
	if _, err := LoadLayoutResources(rootDir, manifest.Resources.Layouts); err != nil {
		return err
	}
	return nil
}

func validateStaticResourcePaths(rootDir string, resources ResourcesConfig) error {
	for _, group := range []struct {
		label string
		paths []string
	}{
		{label: "skill resource", paths: resources.Skills},
		{label: "loop resource", paths: resources.Loops},
		{label: "agent resource", paths: resources.Agents},
		{label: "automation resource", paths: resources.Automation},
		{label: "layout resource", paths: resources.Layouts},
	} {
		for _, path := range group.paths {
			if _, err := resolveBundlePathWithinRoot(rootDir, path, group.label); err != nil {
				return fmt.Errorf("extension: validate declared %s: %w", group.label, err)
			}
		}
	}
	return nil
}
