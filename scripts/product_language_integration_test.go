//go:build integration

package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductLanguageGuard(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		content string
		allowed bool
	}{
		{
			name:    "Should retain product command and header terminology",
			content: "CompozyOS uses compozy. X-Compozy-Session-ID belongs to Compozy Network.\n",
			allowed: true,
		},
		{
			name: "Should accept public API declarations imports types and calls",
			content: `import { Compozy, type ConnectOptions } from "@compozy/client";
export { Compozy as Client } from "@compozy/client/node";
export {
  agents,
  Compozy,
  tasks,
} from "./sdk.gen.js";
export class Compozy extends Client {
  static connect(): Promise<Compozy> { return new Compozy(); }
}
let owner: Compozy | undefined;
const clients: HeyApiRegistry<Compozy> = new HeyApiRegistry<Compozy>();
Compozy.__registry.set(this);
const client = await Compozy.connect();
const started = await Compozy.start();
assert.equal(typeof esm.Compozy.connect, "function");
The API uses ` + "`Compozy.connect()` and `Compozy.start()`" + `.
`,
			allowed: true,
		},
		{
			name: "Should accept generated class naming and wire field identifiers",
			content: "OperationStrategy.single({\n  root: \"Compozy\",\n})\n" +
				"Compozy OpenAIModelCompozyPayload `json:\"compozy\"`\n",
			allowed: true,
		},
		{
			name:    "Should reject retired product prose",
			content: "Compozy daemon is unavailable. Run `compozy daemon start`.\n",
		},
		{
			name:    "Should reject prose beside an allowed API call",
			content: "Compozy.connect() connects to Compozy.\n",
		},
		{
			name:    "Should reject prose beside an allowed import",
			content: "import { Compozy } from \"@compozy/client\"; // Compozy is ready.\n",
		},
		{
			name:    "Should reject product strings in SDK source",
			content: "console.warn(`Compozy client differs from daemon.`);\n",
		},
		{
			name:    "Should reject product sentences beside technical exceptions",
			content: "X-Compozy-Session-ID identifies a Compozy session.\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "copy.mdx")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), "bash", "check-product-language.sh", path)
			output, err := command.CombinedOutput()
			if tc.allowed {
				if err != nil || !strings.Contains(string(output), "Product language is consistent") {
					t.Fatalf("approved terminology rejected: %v\n%s", err, output)
				}
				return
			}
			if err == nil || !strings.Contains(string(output), "Retired product-language") ||
				!strings.Contains(string(output), path) {
				t.Fatalf("retired product prose not reported: %v\n%s", err, output)
			}
		})
	}
}

// TestProductLanguageGuardAgentKind owns UT-020: since v0.5.2 Rule A keeps stays_on out of
// production Go except the rejection-message file, the former alias files included, and Rule B
// keeps the retired word off public surfaces. Each fixture is classified as the repository-relative
// path it stands for.
func TestProductLanguageGuardAgentKind(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, as, content string
		allowed           bool
	}{
		{name: "Should reject stays_on in daemon production code", as: "internal/daemon/x.go",
			content: "package daemon\n\nconst kind = \"stays_on\"\n"},
		{name: "Should reject a CanonicalAgentKind call outside the boundary", as: "internal/daemon/x.go",
			content: "package daemon\n\nfunc f(v string) { config.CanonicalAgentKind(v) }\n"},
		{name: "Should allow the retired spelling in the rejection file", as: "internal/config/agent_kind_retired.go",
			allowed: true, content: "package config\n\nconst retiredAgentKind = \"stays_on\"\n"},
		{name: "Should reject the alias in the former identity alias file", as: "internal/config/agent_identity.go",
			content: "package config\n\nconst alias = \"stays_on\"\n\nfunc CanonicalAgentKind(raw string) {}\n"},
		{name: "Should reject stays_on in the former config table alias file", as: "internal/config/merge.go",
			content: "package config\n\nconst table = \"stays_on\"\n"},
		{name: "Should reject stays_on in the former boot rewrite file", as: "internal/daemon/boot_agent_kind_rewrite.go",
			content: "package daemon\n\nconst kind = \"stays_on\"\n"},
		{name: "Should allow stays_on fixtures in tests", as: "internal/config/agent_test.go", allowed: true,
			content: "package config\n\nconst fixture = \"stays_on\"\n"},
		{name: "Should reject the upper-case spelling", as: "internal/daemon/x.go",
			content: "package daemon\n\nconst STAYS_ON = 1\n"},
		{name: "Should reject the retired word in web copy", as: "web/src/x.tsx",
			content: "export const label = \"stays-on\";\n"},
		{name: "Should allow the English phrase", as: "web/src/x.tsx", allowed: true,
			content: "// the window stays on top\n"},
		{name: "Should allow generated release notes", as: "RELEASE_NOTES.md", allowed: true,
			content: "Renamed stays_on to always_on.\n"},
		{name: "Should allow the site mirror of the migration guide",
			as: "packages/site/content/docs/migration/index.mdx", allowed: true,
			content: "Change `kind: stays_on` to `kind: always_on`.\n"},
		{name: "Should reject the retired word in QA scenarios", as: "docs/qa/scenarios/X.md",
			content: "Open the stays-on row.\n"},
		{name: "Should allow history identifiers in QA scenarios", as: "docs/qa/scenarios/X.md", allowed: true,
			content: "id: MS-web-stays-on-placement\nlast_report: docs/qa/reports/2026-11-03-stays-on-hardening-qa.md\n" +
				"bug_ids: BUG-20261103-link-trigger-stays-on-scope\n"},
		{name: "Should reject prose next to a history identifier", as: "docs/qa/scenarios/X.md",
			content: "See MS-web-stays-on-reports, then open the stays-on row.\n"},
		{name: "Should allow a charter history identifier in QA scenarios", as: "docs/qa/scenarios/X.md",
			allowed: true, content: "charter: CH-web-stays-on-fleet\n"},
		{name: "Should allow the v0.5.2 alias removal release note",
			as: ".release-notes/always-on-aliases-removed-v0.5.2.md", allowed: true,
			content: "Rename `[agents.stays_on]` to `[agents.always_on]`.\n"},
		{name: "Should reject the retired word in another v0.5.1 release fragment",
			as: ".release-notes/other-change-v0.5.1.md", content: "Stays-on agents now pause.\n"},
		{name: "Should reject stays_on in the central agent loader", as: "internal/config/agent.go",
			content: "package config\n\nconst kind = \"stays_on\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), filepath.Base(tc.as))
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), "bash", "check-product-language.sh", path)
			command.Env = append(os.Environ(), "PRODUCT_LANGUAGE_PATH_AS="+tc.as)
			output, err := command.CombinedOutput()
			if tc.allowed {
				if err != nil || !strings.Contains(string(output), "Product language is consistent") {
					t.Fatalf("approved %s rejected: %v\n%s", tc.as, err, output)
				}
				return
			}
			if err == nil || !strings.Contains(string(output), "Retired agent-kind vocabulary") ||
				!strings.Contains(string(output), tc.as) {
				t.Fatalf("retired kind in %s not reported: %v\n%s", tc.as, err, output)
			}
		})
	}
}

// TestProductLanguageGuardAgentKindFailsClosed owns UT-020's fail-closed edge: a ripgrep error in
// the retired-kind scan fails the guard instead of reading as "no match".
func TestProductLanguageGuardAgentKindFailsClosed(t *testing.T) {
	t.Parallel()
	realRG, err := exec.LookPath("rg")
	if err != nil {
		t.Fatalf("rg is required: %v", err)
	}
	dir := t.TempDir()
	wrapper := "#!/usr/bin/env bash\n" +
		"for a in \"$@\"; do [[ \"$a\" == *CanonicalAgentKind* ]] && { echo 'rg: simulated failure' >&2; exit 2; }; done\n" +
		"exec " + realRG + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "rg"), []byte(wrapper), 0o700); err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(dir, "x.md")
	if err := os.WriteFile(fixture, []byte("Plain copy.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), "bash", "check-product-language.sh", fixture)
	command.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "Retired agent-kind scan (Rule A) failed") {
		t.Fatalf("rg failure did not fail the guard: %v\n%s", err, output)
	}
}
