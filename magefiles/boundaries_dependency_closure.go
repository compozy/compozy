//go:build mage

package main

type dependencyClosureRule struct {
	root              string
	allowedPrefixes   []string
	forbiddenExact    []string
	forbiddenPrefixes []string
}

type dependencyClosureViolation struct {
	root       string
	dependency string
}

var subprocessDependencyClosureRules = []dependencyClosureRule{
	{
		root: "./internal/subprocess",
		forbiddenPrefixes: []string{
			compozyModulePath + "internal/store",
			compozyModulePath + "internal/resources",
			compozyModulePath + "internal/task",
			compozyModulePath + "internal/session",
			compozyModulePath + "internal/automation",
			compozyModulePath + "internal/observe",
			compozyModulePath + "internal/extension",
			compozyModulePath + "internal/daemon",
		},
	},
}
