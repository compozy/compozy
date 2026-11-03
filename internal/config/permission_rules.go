package config

import (
	"fmt"
	"path"
	"strings"

	"github.com/bmatcuk/doublestar"
)

// PermissionRule applies only to provider actions submitted for approval.
type PermissionRule struct {
	Kind     string `json:"kind,omitempty" yaml:"kind,omitempty" toml:"kind,omitempty"`
	Path     string `json:"path,omitempty" yaml:"path,omitempty" toml:"path,omitempty"`
	Decision string `json:"decision"       yaml:"decision"       toml:"decision"`
}

func ValidatePermissionRules(rules []PermissionRule) error {
	if len(rules) > 64 {
		return fmt.Errorf("permission_rules must contain at most 64 rules")
	}
	for i, rule := range rules {
		if rule.Decision != "allow" && rule.Decision != "deny" {
			return fmt.Errorf("permission_rules[%d].decision must be allow or deny", i)
		}
		if rule.Kind == "" && rule.Path == "" {
			return fmt.Errorf("permission_rules[%d] requires kind or path", i)
		}
		switch rule.Kind {
		case "", "read", "edit", "delete", "move", "search", "execute", "think", "fetch", "switch_mode", "other":
		default:
			return fmt.Errorf("permission_rules[%d].kind is unknown", i)
		}
		if rule.Path != "" {
			if path.IsAbs(rule.Path) || strings.Contains(rule.Path, "\\") ||
				path.Clean(rule.Path) == ".." || strings.HasPrefix(path.Clean(rule.Path), "../") {
				return fmt.Errorf("permission_rules[%d].path must be relative to the session root", i)
			}
			if _, err := doublestar.Match(rule.Path, ""); err != nil {
				return fmt.Errorf("permission_rules[%d].path: %w", i, err)
			}
		}
	}
	return nil
}
