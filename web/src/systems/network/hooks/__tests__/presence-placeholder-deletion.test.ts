import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

const networkRoot = join(process.cwd(), "src", "systems", "network");
const deletedHookName = ["use", "network", "presence"].join("-");
const deletedHookSymbol = ["use", "Network", "Presence"].join("");
const deletedHookPath = join(networkRoot, "hooks", `${deletedHookName}.ts`);
const currentTestPath = join(
  networkRoot,
  "hooks",
  "__tests__",
  "presence-placeholder-deletion.test.ts"
);

function sourceFiles(root: string): string[] {
  const entries = readdirSync(root);
  const files: string[] = [];
  for (const entry of entries) {
    const path = join(root, entry);
    const stat = statSync(path);
    if (stat.isDirectory()) {
      files.push(...sourceFiles(path));
      continue;
    }
    if (/\.(ts|tsx)$/.test(entry)) {
      files.push(path);
    }
  }
  return files;
}

describe("network presence placeholder deletion", () => {
  it("Should keep the deleted placeholder hook out of the network import graph", () => {
    expect(existsSync(deletedHookPath)).toBe(false);
    for (const file of sourceFiles(networkRoot)) {
      if (file === currentTestPath) {
        continue;
      }
      const source = readFileSync(file, "utf8");
      expect(source, file).not.toContain(deletedHookName);
      expect(source, file).not.toContain(deletedHookSymbol);
    }
  });
});
