package spec

import automationpkg "github.com/compozy/agh/internal/automation"

func automationSourceValues() []string {
	return []string{
		string(automationpkg.JobSourceConfig),
		string(automationpkg.JobSourcePackage),
		string(automationpkg.JobSourceDynamic),
	}
}
