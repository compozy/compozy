package exec

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/execabs"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	defaultExecTimeout   = 30 * time.Second
	defaultStdoutLimit   = 2 << 20
	defaultStderrLimit   = 1 << 10
	defaultCommandMaxArg = 12
)

func loadToolConfig(ctx context.Context) toolConfig {
	cfg := config.DefaultNativeToolsConfig()
	if appCfg := config.FromContext(ctx); appCfg != nil {
		cfg = appCfg.Runtime.NativeTools
	}
	policies := buildCommandPolicies(ctx, cfg.Exec.Allowlist)
	timeout := cfg.Exec.Timeout
	if timeout <= 0 {
		timeout = defaultExecTimeout
	}
	stdoutLimit := cfg.Exec.MaxStdoutBytes
	if stdoutLimit <= 0 {
		stdoutLimit = defaultStdoutLimit
	}
	stderrLimit := cfg.Exec.MaxStderrBytes
	if stderrLimit <= 0 {
		stderrLimit = defaultStderrLimit
	}
	return toolConfig{Timeout: timeout, MaxStdout: stdoutLimit, MaxStderr: stderrLimit, Commands: policies}
}

func buildCommandPolicies(ctx context.Context, overrides []config.NativeExecCommandConfig) map[string]*commandPolicy {
	log := logger.FromContext(ctx)
	policies := make(map[string]*commandPolicy)
	defaults := discoverDefaultCommands(ctx)
	for i := range defaults {
		policy, err := compileCommandConfig(&defaults[i])
		if err != nil {
			log.Warn("Skipping default exec command", "path", defaults[i].Path, "error", err)
			continue
		}
		normalized := normalizePath(policy.Path)
		policies[normalized] = policy
	}
	for i := range overrides {
		policy, err := compileCommandConfig(&overrides[i])
		if err != nil {
			log.Warn("Ignoring invalid exec allowlist override", "path", overrides[i].Path, "error", err)
			continue
		}
		normalized := normalizePath(policy.Path)
		if _, exists := policies[normalized]; exists {
			log.Info("Overriding builtin exec command policy", "path", policy.Path)
		} else {
			log.Info("Appending exec command policy from configuration", "path", policy.Path)
		}
		policies[normalized] = policy
	}
	return policies
}

func discoverDefaultCommands(ctx context.Context) []config.NativeExecCommandConfig {
	log := logger.FromContext(ctx)
	specs := []struct {
		names       []string
		description string
		maxArgs     int
	}{
		{names: []string{"ls"}, description: "List files within a directory", maxArgs: defaultCommandMaxArg},
		{names: []string{"pwd"}, description: "Print current directory", maxArgs: 0},
		{names: []string{"cat"}, description: "Concatenate and print files", maxArgs: defaultCommandMaxArg},
		{names: []string{"grep"}, description: "Search for patterns", maxArgs: defaultCommandMaxArg},
		{names: []string{"find"}, description: "Find files", maxArgs: defaultCommandMaxArg},
		{names: []string{"head"}, description: "Display file start", maxArgs: defaultCommandMaxArg},
		{names: []string{"tail"}, description: "Display file end", maxArgs: defaultCommandMaxArg},
		{names: []string{"echo"}, description: "Echo arguments", maxArgs: defaultCommandMaxArg},
		{names: []string{"stat"}, description: "Show file status", maxArgs: defaultCommandMaxArg},
		{names: []string{"wc"}, description: "Word count", maxArgs: defaultCommandMaxArg},
		{names: []string{"sleep"}, description: "Sleep for duration", maxArgs: 1},
	}
	commands := make([]config.NativeExecCommandConfig, 0, len(specs))
	for _, spec := range specs {
		path, ok := resolveCommandPath(spec.names)
		if !ok {
			log.Info("Default exec command not available on host", "candidates", spec.names)
			continue
		}
		commands = append(commands, config.NativeExecCommandConfig{
			Path:            path,
			Description:     spec.description,
			MaxArgs:         spec.maxArgs,
			AllowAdditional: true,
		})
	}
	return commands
}

func resolveCommandPath(names []string) (string, bool) {
	for _, name := range names {
		path, err := execabs.LookPath(name)
		if err != nil {
			continue
		}
		if filepath.IsAbs(path) {
			return filepath.Clean(path), true
		}
	}
	return "", false
}

func compileCommandConfig(entry *config.NativeExecCommandConfig) (*commandPolicy, error) {
	if entry == nil {
		return nil, errors.New("command configuration is required")
	}
	if strings.TrimSpace(entry.Path) == "" {
		return nil, errors.New("command path is required")
	}
	if !filepath.IsAbs(entry.Path) {
		return nil, fmt.Errorf("command path must be absolute: %s", entry.Path)
	}
	policy := commandPolicy{
		Path:            filepath.Clean(entry.Path),
		Description:     entry.Description,
		Timeout:         entry.Timeout,
		MaxArgs:         entry.MaxArgs,
		AllowAdditional: entry.AllowAdditional,
	}
	if policy.MaxArgs < 0 {
		return nil, fmt.Errorf("command %s max_args cannot be negative", entry.Path)
	}
	if policy.MaxArgs == 0 {
		policy.MaxArgs = defaultCommandMaxArg
	}
	rules, err := compileArgumentRules(entry.Arguments)
	if err != nil {
		return nil, err
	}
	policy.ArgRules = rules
	return &policy, nil
}

func compileArgumentRules(args []config.NativeExecArgumentConfig) ([]argumentRule, error) {
	if len(args) == 0 {
		return nil, nil
	}
	rules := make([]argumentRule, 0, len(args))
	seen := make(map[int]struct{}, len(args))
	for _, cfg := range args {
		if cfg.Index < 0 {
			return nil, fmt.Errorf("argument index must be non-negative")
		}
		if _, exists := seen[cfg.Index]; exists {
			return nil, fmt.Errorf("duplicate argument rule index %d", cfg.Index)
		}
		seen[cfg.Index] = struct{}{}
		rule := argumentRule{Index: cfg.Index, Optional: cfg.Optional}
		if strings.TrimSpace(cfg.Pattern) != "" {
			rx, err := regexp.Compile(cfg.Pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid argument pattern: %w", err)
			}
			rule.Pattern = rx
		}
		if len(cfg.Enum) > 0 {
			enum := make(map[string]struct{}, len(cfg.Enum))
			for _, value := range cfg.Enum {
				enum[value] = struct{}{}
			}
			rule.Enum = enum
		}
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Index < rules[j].Index
	})
	return rules, nil
}

func normalizePath(path string) string {
	clean := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(clean)
	}
	return clean
}
