package command_test

import (
	"errors"
	"strings"
	"testing"

	commandpkg "github.com/compozy/compozy/internal/command"
)

// Suite: session slash command domain
// Invariant: one deterministic catalog controls discovery, parsing, and explicit skill bounds.
// Boundary IN: pure command projection and authored-text grammar.
// Boundary OUT: skill registry state, session persistence, transports, and UI rendering.

func TestBuildCatalog(t *testing.T) {
	t.Parallel()

	t.Run("Should order available commands and reserve daemon names", func(t *testing.T) {
		t.Parallel()
		catalog, err := commandpkg.BuildCatalog(
			commandpkg.DefaultBuiltins(),
			[]commandpkg.AgentSpec{
				{Name: "review", Description: "Agent review", SourceID: "codex"},
				{Name: "goal", Description: "Must lose to the daemon"},
				{Name: "run", Description: "Reserved and hidden"},
			},
			[]commandpkg.SkillSpec{
				{
					Name: "deploy", Description: "Deploy safely", Available: true,
					Source: commandpkg.Source{Kind: "workspace", Scope: "workspace"},
				},
				{
					Name: "inspect", Description: "Inspect extension", Available: true, Qualified: true,
					Source: commandpkg.Source{Kind: "extension", ID: "ops", Scope: "workspace"},
				},
			},
		)
		if err != nil {
			t.Fatalf("BuildCatalog() error = %v", err)
		}
		got := make([]string, 0, len(catalog.Commands))
		for _, descriptor := range catalog.Commands {
			got = append(got, descriptor.CanonicalToken)
		}
		want := []string{"/goal", "/worktree", "/review", "/deploy", "/ops:inspect"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("BuildCatalog() tokens = %#v, want %#v", got, want)
		}
		if catalog.Revision == "" {
			t.Fatal("BuildCatalog() revision is empty")
		}
	})

	t.Run("Should reject external sources that collapse to one token", func(t *testing.T) {
		t.Parallel()
		_, err := commandpkg.BuildCatalog(nil, nil, []commandpkg.SkillSpec{
			{
				Name: "review", Available: true, Qualified: true,
				Source: commandpkg.Source{Kind: "extension", ID: "shared", Scope: "global"},
			},
			{
				Name: "review", Available: true, Qualified: true,
				Source: commandpkg.Source{Kind: "marketplace", ID: "shared", Scope: "global"},
			},
		})
		if err == nil || !strings.Contains(err.Error(), "duplicate skill token") {
			t.Fatalf("BuildCatalog() error = %v, want duplicate token", err)
		}
	})

	t.Run("Should keep marketplace package identity outside the visible qualifier", func(t *testing.T) {
		t.Parallel()
		catalog, err := commandpkg.BuildCatalog(nil, nil, []commandpkg.SkillSpec{{
			Name: "review", Available: true, Qualified: true,
			Source: commandpkg.Source{
				Kind: "marketplace", ID: "official", Key: "official:@compozy/review", Scope: "global",
			},
		}})
		if err != nil {
			t.Fatalf("BuildCatalog() error = %v", err)
		}
		if len(catalog.Commands) != 1 || catalog.Commands[0].CanonicalToken != "/official:review" {
			t.Fatalf("BuildCatalog() commands = %#v, want one /official:review", catalog.Commands)
		}
		if !strings.Contains(catalog.Commands[0].ID, "skill:marketplace:official-") {
			t.Fatalf("BuildCatalog() ID = %q, want package-qualified identity", catalog.Commands[0].ID)
		}
	})

	t.Run("Should keep a disabled builtin visible with its refusal reason", func(t *testing.T) {
		t.Parallel()
		catalog, err := commandpkg.BuildCatalog(commandpkg.DefaultBuiltins(), nil, nil)
		if err != nil {
			t.Fatalf("BuildCatalog() error = %v", err)
		}
		originalRevision := catalog.Revision
		catalog = commandpkg.SetBuiltinAvailability(
			catalog,
			"worktree",
			false,
			"Wait for the current turn to finish.",
		)
		var descriptor *commandpkg.Descriptor
		for index := range catalog.Commands {
			if catalog.Commands[index].CanonicalToken == "/worktree" {
				descriptor = &catalog.Commands[index]
				break
			}
		}
		if descriptor == nil || descriptor.Available ||
			descriptor.UnavailableReason != "Wait for the current turn to finish." {
			t.Fatalf("disabled worktree descriptor = %#v", descriptor)
		}
		if catalog.Revision == originalRevision {
			t.Fatal("disabled worktree catalog revision did not change")
		}
		if _, err := commandpkg.ParseSkillInvocations(
			"/worktree",
			catalog,
		); !errors.Is(
			err,
			commandpkg.ErrUnavailable,
		) {
			t.Fatalf("ParseSkillInvocations(/worktree) error = %v, want ErrUnavailable", err)
		}
		catalog = commandpkg.SetBuiltinAvailability(catalog, "worktree", true, "stale reason")
		for _, command := range catalog.Commands {
			if command.CanonicalToken == "/worktree" && (!command.Available || command.UnavailableReason != "") {
				t.Fatalf("re-enabled worktree descriptor = %#v, want available without refusal reason", command)
			}
		}
	})
}

func TestParseSkillInvocations(t *testing.T) {
	t.Parallel()
	catalog, err := commandpkg.BuildCatalog(commandpkg.DefaultBuiltins(), nil, []commandpkg.SkillSpec{
		{
			Name: "review", Available: true,
			Source: commandpkg.Source{Kind: "workspace", Scope: "workspace"},
		},
		{
			Name: "deploy", Available: true, Qualified: true,
			Source: commandpkg.Source{Kind: "extension", ID: "ops", Scope: "workspace"},
		},
	})
	if err != nil {
		t.Fatalf("BuildCatalog() error = %v", err)
	}

	t.Run("Should resolve multiple inline skills at whitespace boundaries once", func(t *testing.T) {
		t.Parallel()
		invocations, parseErr := commandpkg.ParseSkillInvocations(
			"Please /review, then /ops:deploy and /review again.",
			catalog,
		)
		if parseErr != nil {
			t.Fatalf("ParseSkillInvocations() error = %v", parseErr)
		}
		if len(invocations) != 2 {
			t.Fatalf("ParseSkillInvocations() count = %d, want 2", len(invocations))
		}
		if invocations[0].Token != "/review" || invocations[1].Token != "/ops:deploy" {
			t.Fatalf("ParseSkillInvocations() tokens = %#v", invocations)
		}
	})

	t.Run("Should resolve markers before sentence punctuation", func(t *testing.T) {
		t.Parallel()
		invocations, parseErr := commandpkg.ParseSkillInvocations(
			"First /review. Then /ops:deploy: continue.",
			catalog,
		)
		if parseErr != nil {
			t.Fatalf("ParseSkillInvocations() error = %v", parseErr)
		}
		if len(invocations) != 2 || invocations[0].Token != "/review" ||
			invocations[1].Token != "/ops:deploy" {
			t.Fatalf("ParseSkillInvocations() = %#v, want both punctuated markers", invocations)
		}
	})

	t.Run("Should expose browser-compatible offsets after astral Unicode", func(t *testing.T) {
		t.Parallel()
		invocations, parseErr := commandpkg.ParseSkillInvocations("🚀 /review now", catalog)
		if parseErr != nil {
			t.Fatalf("ParseSkillInvocations() error = %v", parseErr)
		}
		if len(invocations) != 1 || invocations[0].Start != 3 || invocations[0].End != 10 {
			t.Fatalf("ParseSkillInvocations() = %#v, want UTF-16 offsets 3:10", invocations)
		}
	})

	t.Run("Should leave URLs paths in-word and unknown markers literal", func(t *testing.T) {
		t.Parallel()
		invocations, parseErr := commandpkg.ParseSkillInvocations(
			"https://example.com/review ./review path/review /review/file /unknown",
			catalog,
		)
		if parseErr != nil {
			t.Fatalf("ParseSkillInvocations() error = %v", parseErr)
		}
		if len(invocations) != 0 {
			t.Fatalf("ParseSkillInvocations() = %#v, want no invocations", invocations)
		}
	})

	t.Run("Should reject a recognized reserved command", func(t *testing.T) {
		t.Parallel()
		_, parseErr := commandpkg.ParseSkillInvocations("/run now", catalog)
		if !errors.Is(parseErr, commandpkg.ErrUnavailable) {
			t.Fatalf("ParseSkillInvocations() error = %v, want ErrUnavailable", parseErr)
		}
	})

	t.Run("Should prevent a hook from adding an unadmitted skill identity", func(t *testing.T) {
		t.Parallel()
		admitted, parseErr := commandpkg.ParseSkillInvocations("Use /review", catalog)
		if parseErr != nil {
			t.Fatalf("ParseSkillInvocations() error = %v", parseErr)
		}
		reconciled := commandpkg.ReconcileInvocations("Use /ops:deploy only", admitted)
		if len(reconciled) != 0 {
			t.Fatalf("ReconcileInvocations() = %#v, want no widened invocation", reconciled)
		}
	})

	t.Run("Should preserve admitted authored offsets when a hook rewrites surrounding text", func(t *testing.T) {
		t.Parallel()
		admitted, parseErr := commandpkg.ParseSkillInvocations("Use /review here", catalog)
		if parseErr != nil {
			t.Fatalf("ParseSkillInvocations() error = %v", parseErr)
		}
		reconciled := commandpkg.ReconcileInvocations("Hook prefix: use /review here", admitted)
		if len(reconciled) != 1 || reconciled[0].Start != admitted[0].Start ||
			reconciled[0].End != admitted[0].End {
			t.Fatalf("ReconcileInvocations() = %#v, want original authored offsets %#v", reconciled, admitted)
		}
	})
}

func TestBuildInvokedSkillsBlock(t *testing.T) {
	t.Parallel()
	ref := commandpkg.SkillRef{
		CommandID: "skill:workspace::review",
		Name:      "review",
		Source:    commandpkg.Source{Kind: "workspace", Scope: "workspace"},
	}

	t.Run("Should render verified instructions with exact source scope", func(t *testing.T) {
		t.Parallel()
		block, err := commandpkg.BuildInvokedSkillsBlock([]commandpkg.InvokedSkill{
			{Ref: ref, Content: "Review carefully."},
		})
		if err != nil {
			t.Fatalf("BuildInvokedSkillsBlock() error = %v", err)
		}
		if !strings.Contains(block, `command-id="skill:workspace::review"`) ||
			!strings.Contains(block, `scope="workspace"`) ||
			!strings.Contains(block, "Review carefully.") {
			t.Fatalf("BuildInvokedSkillsBlock() = %q, want identity and instructions", block)
		}
	})

	t.Run("Should reject one skill above its explicit budget", func(t *testing.T) {
		t.Parallel()
		_, err := commandpkg.BuildInvokedSkillsBlock([]commandpkg.InvokedSkill{{
			Ref: ref, Content: strings.Repeat("x", commandpkg.MaxSkillBytes+1),
		}})
		if !errors.Is(err, commandpkg.ErrSkillContextTooLarge) {
			t.Fatalf("BuildInvokedSkillsBlock() error = %v, want ErrSkillContextTooLarge", err)
		}
	})

	t.Run("Should reject the combined skill context above its explicit budget", func(t *testing.T) {
		t.Parallel()
		_, err := commandpkg.BuildInvokedSkillsBlock([]commandpkg.InvokedSkill{
			{Ref: ref, Content: strings.Repeat("a", commandpkg.MaxSkillBytes)},
			{
				Ref: commandpkg.SkillRef{
					CommandID: "skill:workspace::deploy",
					Name:      "deploy",
					Source:    ref.Source,
				},
				Content: strings.Repeat("b", commandpkg.MaxSkillBytes),
			},
			{
				Ref: commandpkg.SkillRef{
					CommandID: "skill:workspace::inspect",
					Name:      "inspect",
					Source:    ref.Source,
				},
				Content: strings.Repeat("c", commandpkg.MaxSkillBytes),
			},
		})
		if !errors.Is(err, commandpkg.ErrSkillContextTooLarge) {
			t.Fatalf("BuildInvokedSkillsBlock() error = %v, want combined ErrSkillContextTooLarge", err)
		}
	})
}
