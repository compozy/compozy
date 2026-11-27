package cli

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func TestCompletionPlanCensus(t *testing.T) {
	t.Parallel()

	root := newRootCommand(commandDeps{})

	for path := range completionCommandTable {
		if _, both := completionCommandOptOut[path]; both {
			t.Fatalf("command %q appears in both completionCommandTable and completionCommandOptOut", path)
		}
	}

	seenCommands := map[string]bool{}
	var missing []string

	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		path := completionCommandPath(cmd)
		if path != "" {
			seenCommands[path] = true
		}
		if !cmd.HasSubCommands() && commandHasEntityPlaceholder(cmd) && len(cmd.ValidArgs) == 0 {
			_, inTable := completionCommandTable[path]
			_, inOptOut := completionCommandOptOut[path]
			if !inTable && !inOptOut {
				missing = append(missing, path+" (Use: "+cmd.Use+")")
			}
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)

	if len(missing) > 0 {
		slices.Sort(missing)
		t.Fatalf("leaf commands with placeholders missing from completionCommandTable or completionCommandOptOut:\n%s",
			strings.Join(missing, "\n"))
	}

	for path := range completionCommandTable {
		if !seenCommands[path] {
			t.Errorf("completionCommandTable entry %q does not match any command in root tree", path)
		}
	}
	for path := range completionCommandOptOut {
		if !seenCommands[path] {
			t.Errorf("completionCommandOptOut entry %q does not match any command in root tree", path)
		}
	}
}

func commandHasEntityPlaceholder(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	fields := strings.Fields(cmd.Use)
	if len(fields) <= 1 {
		return false
	}
	rest := strings.Join(fields[1:], " ")
	if rest == "[flags]" {
		return false
	}
	return strings.ContainsAny(rest, "<[")
}

func TestCompletionPlanWiring(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should keep ValidArgsFunction nil on file-only commands and return Default for file arg in mixed commands",
		func(t *testing.T) {
			t.Parallel()

			root := newRootCommand(commandDeps{})
			for _, path := range []string{
				"workspace add",
				"worktree adopt",
				"extension build",
				"extension validate",
				"extension dev",
				"extension publish",
				"app open",
			} {
				cmd, _, err := root.Find(strings.Fields(path))
				if err != nil {
					t.Fatalf("Find(%q) error = %v", path, err)
				}
				if cmd.ValidArgsFunction != nil {
					t.Errorf("%q has non-nil ValidArgsFunction; file args must keep default file completion", path)
				}
			}

			uploadCmd, _, err := root.Find([]string{"session", "attachments", "upload"})
			if err != nil {
				t.Fatalf("Find(session attachments upload) error = %v", err)
			}
			if uploadCmd.ValidArgsFunction == nil {
				t.Fatal("session attachments upload ValidArgsFunction is nil")
			}
			comps, directive := uploadCmd.ValidArgsFunction(uploadCmd, []string{"sess-1"}, "")
			if len(comps) != 0 || directive != cobra.ShellCompDirectiveDefault {
				t.Fatalf("mixed file arg completion = (%v, %v), want (empty, Default)", comps, directive)
			}

			taskRunShow, _, err := root.Find([]string{"task", "run", "show"})
			if err != nil {
				t.Fatalf("Find(task run show) error = %v", err)
			}
			if taskRunShow.ValidArgsFunction == nil {
				t.Fatal("task run show ValidArgsFunction is nil")
			}
			idComps, idDirective := taskRunShow.ValidArgsFunction(taskRunShow, nil, "")
			if len(idComps) != 0 || idDirective != cobra.ShellCompDirectiveNoFileComp {
				t.Fatalf("opted-out id completion = (%v, %v), want (empty, NoFileComp)", idComps, idDirective)
			}
		},
	)

	t.Run("Should exclude hidden --profile on machine-wide commands", func(t *testing.T) {
		t.Parallel()

		root := newRootCommand(commandDeps{})
		for _, path := range []string{
			"daemon",
			"daemon start",
			"daemon profile",
			"install",
			"update",
			"completion",
			"completion bash",
		} {
			cmd, _, err := root.Find(strings.Fields(path))
			if err != nil {
				t.Fatalf("Find(%q) error = %v", path, err)
			}
			if _, ok := cmd.GetFlagCompletionFunc(profileFlagName); ok {
				t.Errorf("%q should not register completion for hidden --profile", path)
			}
		}

		sessionList, _, err := root.Find([]string{"session", "list"})
		if err != nil {
			t.Fatalf("Find(session list) error = %v", err)
		}
		if _, ok := sessionList.GetFlagCompletionFunc(profileFlagName); !ok {
			t.Fatal("session list should register completion for --profile")
		}
	})

	t.Run("Should wire provider models set arg0 to provider and arg1 to models of that provider", func(t *testing.T) {
		t.Parallel()

		var requestedProvider string
		stub := &stubClient{
			listProvidersFn: func(context.Context) (contract.ProviderListResponse, error) {
				return contract.ProviderListResponse{
					Providers: []contract.ProviderSummaryPayload{{Name: "openai", DisplayName: "OpenAI"}},
				}, nil
			},
			listProviderModelsFn: func(_ context.Context, q ProviderModelListQuery) (ProviderModelListRecord, error) {
				requestedProvider = q.ProviderID
				return ProviderModelListRecord{
					Models: []ProviderModelRecord{{ModelID: "gpt-5", DisplayName: "GPT-5"}},
				}, nil
			},
		}
		root := newRootCommand(newTestDeps(t, stub))
		setCmd, _, err := root.Find([]string{"provider", "models", "set"})
		if err != nil {
			t.Fatalf("Find(provider models set) error = %v", err)
		}
		if setCmd.ValidArgsFunction == nil {
			t.Fatal("provider models set ValidArgsFunction is nil")
		}

		pComps, pDir := setCmd.ValidArgsFunction(setCmd, nil, "")
		if !slices.Equal(pComps, []string{"openai\tOpenAI"}) || pDir != cobra.ShellCompDirectiveNoFileComp {
			t.Fatalf("arg0 = (%#v, %v), want ([openai\\tOpenAI], NoFileComp)", pComps, pDir)
		}

		mComps, mDir := setCmd.ValidArgsFunction(setCmd, []string{"openai"}, "")
		if requestedProvider != "openai" {
			t.Fatalf("ListProviderModels ProviderID = %q, want %q", requestedProvider, "openai")
		}
		if !slices.Equal(mComps, []string{"gpt-5\tGPT-5"}) || mDir != cobra.ShellCompDirectiveNoFileComp {
			t.Fatalf("arg1 = (%#v, %v), want ([gpt-5\\tGPT-5], NoFileComp)", mComps, mDir)
		}

		extraComps, extraDir := setCmd.ValidArgsFunction(setCmd, []string{"openai", "gpt-5"}, "")
		if len(extraComps) != 0 || extraDir != cobra.ShellCompDirectiveDefault {
			t.Fatalf("arg2 = (%#v, %v), want (nil, Default)", extraComps, extraDir)
		}
	})

	t.Run("Should respect per-command flag family overrides", func(t *testing.T) {
		t.Parallel()

		root := newRootCommand(commandDeps{})

		accountLogin, _, err := root.Find([]string{"account", "login"})
		if err != nil {
			t.Fatalf("Find(account login) error = %v", err)
		}
		if _, ok := accountLogin.GetFlagCompletionFunc("provider"); ok {
			t.Error("account login --provider should be opted out")
		}

		supportBundle, _, err := root.Find([]string{"support", "bundle"})
		if err != nil {
			t.Fatalf("Find(support bundle) error = %v", err)
		}
		if _, ok := supportBundle.GetFlagCompletionFunc("output"); ok {
			t.Error("support bundle --output should be opted out (file path)")
		}

		wsAdd, _, err := root.Find([]string{"workspace", "add"})
		if err != nil {
			t.Fatalf("Find(workspace add) error = %v", err)
		}
		if _, ok := wsAdd.GetFlagCompletionFunc("name"); ok {
			t.Error("workspace add --name should not have loop completion")
		}

		loopInspect, _, err := root.Find([]string{"loop", "inspect"})
		if err != nil {
			t.Fatalf("Find(loop inspect) error = %v", err)
		}
		if _, ok := loopInspect.GetFlagCompletionFunc("name"); !ok {
			t.Error("loop inspect --name should register loop completion")
		}

		taskProfileUpdate, _, err := root.Find([]string{"task", "profile", "update"})
		if err != nil {
			t.Fatalf("Find(task profile update) error = %v", err)
		}
		if _, ok := taskProfileUpdate.GetFlagCompletionFunc(profileFlagName); ok {
			t.Error("task profile update --profile should be opted out (JSON payload)")
		}
	})
}
