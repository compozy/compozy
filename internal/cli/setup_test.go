package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/compozy/looper/internal/setup"
	"github.com/spf13/cobra"
)

func TestSetupHelpShowsSetupFlagsOnly(t *testing.T) {
	t.Parallel()

	output, err := executeRootCommand("setup", "--help")
	if err != nil {
		t.Fatalf("execute setup help: %v", err)
	}

	required := []string{"--agent", "--skill", "--global", "--copy", "--list", "--yes", "--all"}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected setup help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{"--provider", "--pr", "--tasks-dir", "--batch-size", "--concurrent"}
	for _, snippet := range forbidden {
		if strings.Contains(output, snippet) {
			t.Fatalf("expected setup help to omit %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestSetupRunYesFailsWithoutDetectedAgents(t *testing.T) {
	t.Parallel()

	state := newSetupCommandState()
	state.listSkills = func() ([]setup.Skill, error) {
		return []setup.Skill{{Name: "create-prd", Description: "Create a PRD"}}, nil
	}
	state.listAgents = func(setup.ResolverOptions) ([]setup.Agent, error) {
		return []setup.Agent{
			{
				Name:           "codex",
				DisplayName:    "Codex",
				ProjectRootDir: ".agents/skills",
				GlobalRootDir:  ".codex/skills",
				Universal:      true,
			},
		}, nil
	}
	state.detectAgents = func(setup.ResolverOptions) ([]setup.Agent, error) {
		return nil, nil
	}
	state.yes = true

	cmd := &cobra.Command{Use: "setup"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().Bool("global", false, "global")
	cmd.Flags().Bool("copy", false, "copy")

	err := state.run(cmd, nil)
	if err == nil {
		t.Fatal("expected setup run to fail when no agents are detected")
	}
	if !strings.Contains(err.Error(), "no agents detected") {
		t.Fatalf("expected missing detected agents error, got %v", err)
	}
}
