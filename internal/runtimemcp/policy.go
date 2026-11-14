package runtimemcp

import (
	"cmp"
	"path"
	"regexp"
	"slices"
	"strings"
)

func matchesNames(patterns any, name string) bool {
	for _, pattern := range names(patterns) {
		matched, err := path.Match(pattern, name)
		if err == nil && matched {
			return true
		}
	}
	return false
}

func (r *reader) applyMCPPolicy(runtime string, policy object, entries map[string]entry) {
	for name, item := range entries {
		if matchesNames(policy["excluded"], name) || (has(policy, "allowed") && !matchesNames(policy["allowed"], name)) {
			item.server.EnabledInRuntime = false
		}
		entries[name] = item
	}
}

func claudePolicyMatches(rules any, name string, d object) bool {
	list, _ := rules.([]any)
	command := []string{str(d["command"])}
	command = append(command, names(d["args"])...)
	endpoint := cmp.Or(str(d["url"]), str(d["httpUrl"]))
	for _, rule := range list {
		entry := obj(rule)
		if str(entry["serverName"]) == name {
			return true
		}
		if has(entry, "serverCommand") && slices.Equal(names(entry["serverCommand"]), command) {
			return true
		}
		if pattern := str(entry["serverUrl"]); pattern != "" && endpoint != "" {
			expression := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*") + "$"
			if matched, err := regexp.MatchString(expression, endpoint); err == nil && matched {
				return true
			}
		}
	}
	return false
}

func applyClaudePolicy(settings object, entries map[string]entry) {
	for name, item := range entries {
		allowed := !has(settings, "allowedMcpServers") || claudePolicyMatches(settings["allowedMcpServers"], name, item.definition)
		if !allowed || claudePolicyMatches(settings["deniedMcpServers"], name, item.definition) {
			item.server.EnabledInRuntime = false
			item.server.Approval = "rejected"
		}
		entries[name] = item
	}
}
