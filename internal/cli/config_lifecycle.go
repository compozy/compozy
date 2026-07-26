package cli

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	configlifecycle "github.com/compozy/agh/internal/config/lifecycle"
)

type configMutationLifecycle struct {
	Lifecycle       string
	Applied         bool
	RestartRequired bool
	RestartScope    string
	NextAction      string
}

func classifyConfigSetLifecycle(path []string) configMutationLifecycle {
	field := strings.Join(path, ".")
	rule, err := configlifecycle.ClassifyPath(field)
	if err != nil {
		return restartRequiredConfigLifecycle()
	}
	return configLifecycleFromRule(rule)
}

func configLifecycleFromRule(rule configlifecycle.Rule) configMutationLifecycle {
	if rule.Lifecycle == configlifecycle.RestartRequired {
		return restartRequiredConfigLifecycle()
	}
	nextAction := contract.SettingsApplyNextActionNone
	return configMutationLifecycle{
		Lifecycle:  string(rule.Lifecycle),
		Applied:    true,
		NextAction: string(nextAction),
	}
}

func restartRequiredConfigLifecycle() configMutationLifecycle {
	return configMutationLifecycle{
		Lifecycle:       string(contract.SettingsApplyLifecycleRestartRequired),
		RestartRequired: true,
		RestartScope:    configDaemonKey,
		NextAction:      string(contract.SettingsApplyNextActionRestartDaemon),
	}
}
