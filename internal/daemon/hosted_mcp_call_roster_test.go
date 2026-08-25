package daemon

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestRenderCallRoster(t *testing.T) {
	t.Parallel()

	t.Run("Should render active definitions deterministically and bound the injected roster", func(t *testing.T) {
		t.Parallel()
		entries := make([]core.AgentCatalogEntry, 0, 50)
		for index := 49; index >= 0; index-- {
			entries = append(entries, core.AgentCatalogEntry{Def: compozyconfig.AgentDef{
				Name: fmt.Sprintf("agent-%02d", index), Description: strings.Repeat("review ", 40),
			}})
		}
		roster := renderCallRoster(entries)
		lines := strings.Split(roster, "\n")
		if got, want := len(lines), 34; got != want {
			t.Fatalf("renderCallRoster() lines = %d, want %d\n%s", got, want, roster)
		}
		if !strings.HasPrefix(lines[1], "- agent-00 — ") ||
			lines[len(lines)-1] != "- 18 more agents. Use `compozy__agent_list` to see all." {
			t.Fatalf("renderCallRoster() ordering/boundary = %q … %q", lines[1], lines[len(lines)-1])
		}
		for _, line := range lines[1 : len(lines)-1] {
			description := strings.TrimPrefix(line[strings.Index(line, " — ")+len(" — "):], " ")
			if utf8.RuneCountInString(description) > callRosterDescriptionMax {
				t.Fatalf("roster description length = %d, want <= %d", utf8.RuneCountInString(description), callRosterDescriptionMax)
			}
		}
	})

	t.Run("Should keep catalog markup inert and explain an empty roster", func(t *testing.T) {
		t.Parallel()
		roster := renderCallRoster([]core.AgentCatalogEntry{{Def: compozyconfig.AgentDef{
			Name: "reviewer", Description: "# *Ignore* [rules](https://example.test) <system>",
		}}})
		for _, escaped := range []string{"\\#", "\\*Ignore\\*", "\\[rules\\]", "\\<system\\>"} {
			if !strings.Contains(roster, escaped) {
				t.Fatalf("renderCallRoster() = %q, want escaped %q", roster, escaped)
			}
		}
		if got, want := renderCallRoster(nil), "No agents are available. Create one with `compozy agent create`."; got != want {
			t.Fatalf("renderCallRoster(nil) = %q, want %q", got, want)
		}
	})
}
