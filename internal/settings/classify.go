package settings

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/config/lifecycle"
)

const (
	classifyConsolidateKey = "consolidate"
	classifyRestartKey     = "restart"
	restartScopeDaemon     = "daemon"
)

// ClassifyMutation maps one section or collection mutation onto the v1 runtime-apply matrix.
func ClassifyMutation(descriptor MutationDescriptor) (MutationClassification, error) {
	section := descriptor.Section
	if section == "" {
		return MutationClassification{}, errors.New("settings: mutation section is required")
	}

	action := strings.TrimSpace(descriptor.Action)
	if action != "" {
		return classifyAction(section, action)
	}
	if len(descriptor.ChangedFields) == 0 {
		return MutationClassification{}, errors.New("settings: mutation fields or action are required")
	}

	configLifecycle, diffClass, err := lifecycle.ClassifyPaths(descriptor.ChangedFields)
	if err != nil {
		return MutationClassification{}, err
	}
	return classificationFromLifecycle(configLifecycle, diffClass), nil
}

func classifyAction(section SectionName, action string) (MutationClassification, error) {
	switch section {
	case SectionGeneral:
		if action == classifyRestartKey {
			return MutationClassification{
				Behavior:  MutationBehaviorActionTrigger,
				Applied:   true,
				Lifecycle: lifecycle.Live,
				DiffClass: lifecycle.DiffClassForRoot(string(section)),
			}, nil
		}
	case SectionMemory:
		if action == classifyConsolidateKey {
			return MutationClassification{
				Behavior:  MutationBehaviorActionTrigger,
				Applied:   true,
				Lifecycle: lifecycle.Live,
				DiffClass: lifecycle.DiffClassForRoot(string(section)),
			}, nil
		}
	case SectionHooksExtensions:
		switch action {
		case "extension-install", "extension-enable", "extension-disable":
			return MutationClassification{
				Behavior:  MutationBehaviorActionTrigger,
				Applied:   true,
				Lifecycle: lifecycle.Live,
				DiffClass: lifecycle.DiffClassForRoot(string(section)),
			}, nil
		}
	}

	return MutationClassification{}, fmt.Errorf("settings: unsupported action %q for section %q", action, section)
}

func restartRequiredClassification() MutationClassification {
	return MutationClassification{
		Behavior:        MutationBehaviorRestartRequired,
		RestartRequired: true,
		RestartScope:    restartScopeDaemon,
		Lifecycle:       lifecycle.RestartRequired,
		DiffClass:       lifecycle.DiffClassRestartRequired,
	}
}

func classificationFromLifecycle(
	configLifecycle lifecycle.Lifecycle,
	diffClass lifecycle.DiffClass,
) MutationClassification {
	switch configLifecycle {
	case lifecycle.RestartRequired:
		return MutationClassification{
			Behavior:        MutationBehaviorRestartRequired,
			RestartRequired: true,
			RestartScope:    restartScopeDaemon,
			Lifecycle:       configLifecycle,
			DiffClass:       diffClass,
		}
	default:
		return MutationClassification{
			Behavior:  MutationBehaviorAppliedNow,
			Applied:   true,
			Lifecycle: configLifecycle,
			DiffClass: diffClass,
		}
	}
}
