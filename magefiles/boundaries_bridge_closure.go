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

var bridgeDependencyClosureRules = func() []dependencyClosureRule {
	allowedPrefixes := []string{compozyModulePath + "internal/bridges/contract"}
	forbiddenPrefixes := []string{
		compozyModulePath + "internal/bridges",
		compozyModulePath + "internal/store",
		compozyModulePath + "internal/resources",
		compozyModulePath + "internal/task",
		compozyModulePath + "internal/session",
		compozyModulePath + "internal/automation",
		compozyModulePath + "internal/observe",
		compozyModulePath + "internal/extension",
		compozyModulePath + "internal/daemon",
	}
	roots := []string{
		"./internal/bridgesdk",
		"./internal/subprocess",
		"./extensions/bridges/slack",
		"./extensions/bridges/telegram",
		"./extensions/bridges/discord",
		"./extensions/bridges/teams",
		"./extensions/bridges/gchat",
		"./extensions/bridges/whatsapp",
		"./extensions/bridges/github",
		"./extensions/bridges/linear",
		"./internal/extension/testdata/telegram-reference",
	}
	rules := make([]dependencyClosureRule, 0, len(roots))
	for _, root := range roots {
		rules = append(rules, dependencyClosureRule{
			allowedPrefixes:   allowedPrefixes,
			root:              root,
			forbiddenPrefixes: forbiddenPrefixes,
		})
	}
	return rules
}()
