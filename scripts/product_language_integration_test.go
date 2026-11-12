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
// removed-input diagnostics, and Rule B keeps the retired word off public surfaces. Each
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
		{name: "Should reject the alias in the identity file", as: "internal/config/agent_identity.go",
			content: "package config\n\nconst alias = \"stays_on\"\n\nfunc CanonicalAgentKind(raw string) {}\n"},
		{name: "Should reject an alias comparison outside the removed diagnostic", as: "internal/config/alias.go",
			content: "package config\nfunc alias(path []string) bool { return path[1] == \"stays_on\" }\n"},
		{name: "Should allow the removed-config comparison only in its diagnostic owner", as: "internal/config/removed_agent_config.go", allowed: true,
			content: "package config\nfunc removed(path []string) bool { return path[1] == \"stays_on\" }\n"},
		{name: "Should reject alias constants inside the diagnostic owner", as: "internal/config/removed_agent_config.go",
			content: "package config\nconst kind = \"stays_on\"\n"},

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

// TestProductLanguageGuardRetiredIdentity owns brand UT-012: Rule C keeps the retired orange accent
// and the Inter/Playfair/JetBrains Mono default stacks out of product, site, video and README
// sources, while user-chosen font catalogs and negative assertions stay legal.
func TestProductLanguageGuardRetiredIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, as, content string
		allowed           bool
	}{
		{name: "Should reject the retired orange hex in web source", as: "web/src/x.tsx",
			content: "export const accent = \"#E8572A\";\n"},
		{name: "Should reject the retired orange rgba in site styles", as: "packages/site/app/global.css",
			content: ".x { box-shadow: 0 0 0 1px rgba(232, 87, 42, 0.4); }\n"},
		{name: "Should reject a retired sans default stack", as: "packages/ui/src/tokens.css",
			content: ":root {\n  --font-sans:\n    \"Inter Variable\", -apple-system,\n    sans-serif;\n}\n"},
		{name: "Should reject a retired display face in an OG font family", as: "packages/site/lib/og/templates/x.tsx",
			content: "const style = { fontFamily: \"Playfair Display\", fontSize: 64 };\n"},
		{name: "Should reject a retired mono default", as: "web/src/systems/theme/lib/defaults.ts",
			content: "export const DEFAULT_FONT_LABEL = {\n  sans: \"Figtree\",\n  mono: \"JetBrains Mono\",\n};\n"},
		{name: "Should reject a retired stack in the video brand fonts", as: "packages/launch-video/src/brand/fonts.ts",
			content: "export const mono = { fontFamily: \"JetBrains Mono, monospace\" };\n"},
		{name: "Should reject the retired orange in the README", as: "README.md",
			content: "<p style=\"color: #e8572a\">CompozyOS</p>\n"},
		{name: "Should allow the current defaults", as: "packages/ui/src/tokens.css", allowed: true,
			content: ":root {\n  --font-sans: \"Figtree Variable\", sans-serif;\n  --font-mono: \"Geist Mono Variable\", monospace;\n  --color-brand: #2f5bff;\n}\n"},
		{name: "Should reject a retired default in the Appearance catalog owner", as: "web/src/systems/theme/lib/font-appearance.ts",
			content: "export const FONT_CATALOG = [{ family: \"JetBrains Mono Variable\" }];\n" +
				"const DEFAULT_FONT_FAMILY: Record<FontRole, string> = {\n  sans: \"Inter Variable\",\n  mono: \"Geist Mono Variable\",\n};\n"},
		{name: "Should reject the retired orange in the Appearance catalog owner", as: "web/src/systems/theme/lib/font-appearance.ts",
			content: "export const FONT_CATALOG = [{ family: \"JetBrains Mono Variable\" }];\nconst swatch = \"#e8572a\";\n"},
		{name: "Should allow the Appearance catalog and its loaders", as: "web/src/systems/theme/lib/font-appearance.ts",
			allowed: true, content: "export const FONT_CATALOG = [{ family: \"JetBrains Mono Variable\" }];\nconst fontFamily = \"JetBrains Mono Variable\";\n"},
		{name: "Should allow negative assertions in the OG suite", as: "packages/site/lib/__tests__/opengraph-image.test.tsx",
			allowed: true, content: "const RETIRED_HEXES = [\"#E8572A\"];\nexpect(fontFamily).not.toContain(\"Playfair Display\");\n"},
		{name: "Should allow the identity palette", as: "packages/ui/src/lib/identity-palette.ts", allowed: true,
			content: "export const LEGACY = \"#e8572a\";\n"},
		{name: "Should allow retired fonts named in prose outside a stack", as: "packages/brand/src/manual/product/x.ts",
			allowed: true, content: "const rows = [\n  [\"--font-display: logo only\"],\n  [\"OG mono\", \"JetBrains Mono\", \"Geist Mono\"],\n];\n"},
		{name: "Should allow design archives outside the scanned trees", as: "docs/design/archive.md", allowed: true,
			content: "The v1 accent was #E8572A with Inter Variable.\n"},
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
			if err == nil || !strings.Contains(string(output), "Retired brand identity found") ||
				!strings.Contains(string(output), tc.as) {
				t.Fatalf("retired identity in %s not reported: %v\n%s", tc.as, err, output)
			}
		})
	}
}

// TestProductLanguageGuardRetiredIdentityFailsClosed owns UT-012's fail-closed edge: a ripgrep
// error in the retired-identity scan fails the guard instead of reading as "no match".
func TestProductLanguageGuardRetiredIdentityFailsClosed(t *testing.T) {
	t.Parallel()
	realRG, err := exec.LookPath("rg")
	if err != nil {
		t.Fatalf("rg is required: %v", err)
	}
	dir := t.TempDir()
	wrapper := "#!/usr/bin/env bash\n" +
		"for a in \"$@\"; do [[ \"$a\" == *e8572a* ]] && { echo 'rg: simulated failure' >&2; exit 2; }; done\n" +
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
	if err == nil || !strings.Contains(string(output), "Retired agent-kind scan (Rule C) failed") {
		t.Fatalf("rg failure did not fail the guard: %v\n%s", err, output)
	}
}
