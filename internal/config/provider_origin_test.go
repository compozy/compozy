package config

import (
	"encoding/json"
	"strings"
	"testing"

	burnttoml "github.com/BurntSushi/toml"
)

func TestCommandIsBuiltinOrigin(t *testing.T) {
	t.Parallel()

	t.Run("Should mark inherited builtin and clear explicit copies", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{}
		builtin, err := cfg.ResolveProvider("opencode")
		if err != nil {
			t.Fatalf("ResolveProvider(opencode) error = %v", err)
		}
		if builtin.Command != "opencode acp" || !builtin.CommandIsBuiltin {
			t.Fatalf("builtin = %#v, want opencode acp with origin", builtin)
		}
		explicit := &Config{Providers: map[string]ProviderConfig{"opencode": {Command: "opencode acp"}}}
		authored, err := explicit.ResolveProvider("opencode")
		if err != nil {
			t.Fatalf("ResolveProvider(explicit copy) error = %v", err)
		}
		if authored.CommandIsBuiltin {
			t.Fatal("explicit copy retained builtin origin")
		}
	})

	t.Run("Should propagate origin to agents and clear overrides", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{}
		agent, err := cfg.ResolveAgent(AgentDef{Name: "a", Provider: "opencode", Prompt: "p"})
		if err != nil {
			t.Fatalf("ResolveAgent() error = %v", err)
		}
		if !agent.CommandIsBuiltin {
			t.Fatalf("agent = %#v, want inherited origin", agent)
		}
		overridden, err := cfg.ResolveAgent(
			AgentDef{Name: "a", Provider: "opencode", Command: "opencode acp", Prompt: "p"},
		)
		if err != nil {
			t.Fatalf("ResolveAgent(explicit) error = %v", err)
		}
		if overridden.CommandIsBuiltin {
			t.Fatal("agent explicit command retained origin")
		}
		routed, err := cfg.ResolveSessionAgentWithRuntime(
			AgentDef{Name: "a", Provider: "opencode", Prompt: "p"},
			RuntimeOverrides{Command: "opencode acp"},
		)
		if err != nil {
			t.Fatalf("ResolveSessionAgentWithRuntime() error = %v", err)
		}
		if routed.CommandIsBuiltin {
			t.Fatal("route override retained origin")
		}
	})

	t.Run("Should preserve origin through clone and hide from serialization", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{}
		provider, err := cfg.ResolveProvider("opencode")
		if err != nil {
			t.Fatalf("ResolveProvider() error = %v", err)
		}
		if !CloneProviderConfig(provider).CommandIsBuiltin {
			t.Fatal("clone lost origin")
		}
		payload, err := json.Marshal(provider)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if strings.Contains(string(payload), "CommandIsBuiltin") ||
			strings.Contains(string(payload), "commandIsBuiltin") {
			t.Fatalf("payload = %s, want origin hidden", payload)
		}
		tomlPayload, err := burnttoml.Marshal(provider)
		if err != nil {
			t.Fatalf("toml.Marshal() error = %v", err)
		}
		if strings.Contains(string(tomlPayload), "CommandIsBuiltin") ||
			strings.Contains(string(tomlPayload), "commandisbuiltin") {
			t.Fatalf("toml = %s, want origin hidden", tomlPayload)
		}
		agent, err := cfg.ResolveAgent(AgentDef{Name: "a", Provider: "opencode", Prompt: "p"})
		if err != nil {
			t.Fatalf("ResolveAgent() error = %v", err)
		}
		agentPayload, err := json.Marshal(agent)
		if err != nil {
			t.Fatalf("Marshal(agent) error = %v", err)
		}
		if strings.Contains(string(agentPayload), "CommandIsBuiltin") ||
			strings.Contains(string(agentPayload), "commandIsBuiltin") {
			t.Fatalf("agent payload = %s, want origin hidden", agentPayload)
		}
	})
}
