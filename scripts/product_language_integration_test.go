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
