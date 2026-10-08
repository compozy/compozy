package spec

import automationpkg "github.com/compozy/compozy/internal/automation"

func automationSourceValues() []string {
	return []string{
		string(automationpkg.JobSourceConfig),
		string(automationpkg.JobSourcePackage),
		string(automationpkg.JobSourceDynamic),
	}
}

// automationTargetFilterValues lists the list-filter target kinds. "task" is a
// filter-only value: jobs with a task target.
func automationTargetFilterValues() []string {
	return []string{
		string(automationpkg.TargetKindAgent),
		string(automationpkg.TargetKindLoop),
		"task",
	}
}
