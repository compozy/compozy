package providerdefaults

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/provider"
)

func TestDefaultRegistryForWorkspaceRunsCodeRabbitCommandsFromWorkspaceRoot(t *testing.T) {
	t.Run("Should run CodeRabbit commands from workspace root", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		otherRoot := t.TempDir()
		fakeBin := t.TempDir()
		fakeGH := filepath.Join(fakeBin, "gh")
		script := strings.Join([]string{
			"#!/bin/sh",
			"set -eu",
			`case "$1 $2" in`,
			`  "repo view")`,
			`    if [ "$PWD" = "$WORKSPACE_ROOT" ]; then`,
			`      printf '{"owner":{"login":"acme"},"name":"target"}'`,
			`    else`,
			`      printf '{"owner":{"login":"wrong"},"name":"source"}'`,
			`    fi`,
			`    ;;`,
			`  "api repos/acme/target/pulls/259/comments?per_page=100&page=1")`,
			`    printf '[{"id":101,"node_id":"RC_101","body":"Workspace scoped comment","path":"internal/right.go","line":7,"user":{"login":"coderabbitai[bot]"}}]'`,
			`    ;;`,
			`  "api graphql")`,
			`    printf '{"data":{"repository":{"pullRequest":{"reviewThreads":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[{"id":"PRT_workspace","isResolved":false,"comments":{"nodes":[{"id":"comment-101","databaseId":101}]}}]}}}}}'`,
			`    ;;`,
			`  *)`,
			`    echo "unexpected gh invocation from $PWD: $*" >&2`,
			`    exit 7`,
			`    ;;`,
			`esac`,
			"",
		}, "\n")
		if err := os.WriteFile(fakeGH, []byte(script), 0o700); err != nil {
			t.Fatalf("write fake gh: %v", err)
		}

		t.Setenv("WORKSPACE_ROOT", workspaceRoot)
		t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
		t.Chdir(otherRoot)

		registry := DefaultRegistryForWorkspace(workspaceRoot)
		reviewProvider, err := registry.Get("coderabbit")
		if err != nil {
			t.Fatalf("registry.Get(coderabbit): %v", err)
		}
		items, err := reviewProvider.FetchReviews(context.Background(), provider.FetchRequest{PR: "259"})
		if err != nil {
			t.Fatalf("fetch reviews: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("expected one workspace-scoped item, got %d (%#v)", len(items), items)
		}
		if items[0].File != "internal/right.go" || items[0].ProviderRef != "thread:PRT_workspace,comment:RC_101" {
			t.Fatalf("unexpected fetched item: %#v", items[0])
		}
	})
}
