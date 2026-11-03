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

// TestProductLanguageGuardAgentKind owns UT-020: Rule A keeps stays_on and CanonicalAgentKind at
// the listed production alias files, and Rule B keeps the retired word off public surfaces. Each
// fixture is classified as the repository-relative path it stands for.
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
		{name: "Should allow the alias in the identity file", as: "internal/config/agent_identity.go", allowed: true,
			content: "package config\n\nconst alias = \"stays_on\"\n\nfunc CanonicalAgentKind(raw string) {}\n"},
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
		{name: "Should reject the retired word in QA scenarios", as: "docs/qa/scenarios/X.md",
			content: "Open the stays-on row.\n"},
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
