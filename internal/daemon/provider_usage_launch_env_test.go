package daemon

import (
	"maps"
	"slices"
	"testing"

	config "github.com/compozy/compozy/internal/config"
)

func TestApplyLaunchEnvironment(t *testing.T) {
	t.Parallel()
	daemonEnv := []string{"HOME=/daemon", "ARGV0=claude", "CLAUDE_CONFIG_DIR=/inherited"}
	cases := []struct {
		name    string
		command string
		owned   []string
		env     []string
	}{
		{
			name:    "Should own a leading selector before an env prefix UT-001",
			command: "CLAUDE_CONFIG_DIR=/b env -u ARGV0 claude-agent-acp",
			owned:   []string{"CLAUDE_CONFIG_DIR"},
			env:     []string{"HOME=/daemon", "CLAUDE_CONFIG_DIR=/b"},
		},
		{
			name:    "Should own a selector assigned inside the env prefix UT-001",
			command: "env -u ARGV0 CLAUDE_CONFIG_DIR=/b claude-agent-acp",
			owned:   []string{"CLAUDE_CONFIG_DIR"},
			env:     []string{"HOME=/daemon", "CLAUDE_CONFIG_DIR=/b"},
		},
		{
			name:    "Should not own an empty assignment UT-002",
			command: "CLAUDE_CONFIG_DIR= claude-agent-acp",
			env:     []string{"HOME=/daemon", "ARGV0=claude", "CLAUDE_CONFIG_DIR="},
		},
		{
			name:    "Should drop ownership of a later unset UT-002",
			command: "env CLAUDE_CONFIG_DIR=/x -u CLAUDE_CONFIG_DIR claude-agent-acp",
			env:     []string{"HOME=/daemon", "ARGV0=claude"},
		},
		{
			name:    "Should not own an inherited value removed by env -u UT-002",
			command: "env -u CLAUDE_CONFIG_DIR claude-agent-acp",
			env:     []string{"HOME=/daemon", "ARGV0=claude"},
		},
		{
			name:    "Should own only assignments after env -i UT-003",
			command: "CODEX_HOME=/lost env -i HOME=/h CLAUDE_CONFIG_DIR=/b claude-agent-acp",
			owned:   []string{"CLAUDE_CONFIG_DIR", "HOME"},
			env:     []string{"HOME=/h", "CLAUDE_CONFIG_DIR=/b"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parsed, err := config.ParseLaunchCommand(tc.command)
			if err != nil {
				t.Fatal(err)
			}
			env, owned, err := applyLaunchEnvironment(slices.Clone(daemonEnv), parsed)
			if err != nil {
				t.Fatal(err)
			}
			if got := slices.Sorted(maps.Keys(owned)); !slices.Equal(got, tc.owned) {
				t.Fatalf("owned = %v, want %v", got, tc.owned)
			}
			if !slices.Equal(env, tc.env) {
				t.Fatalf("env = %v, want %v", env, tc.env)
			}
		})
	}
}
