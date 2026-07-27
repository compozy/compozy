import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const repoRoot = resolve(siteRoot, "../..");
const landingRoot = resolve(siteRoot, "components/landing");
const runtimeRoot = resolve(siteRoot, "content/runtime");
const comparisonPath = resolve(landingRoot, "comparison.tsx");

const approvedMarketSources = new Set([
  ".resources/orca/src/main/ghostty/index.ts",
  ".resources/mastra/mastracode/factory/README.md",
  ".resources/paperclip/README.md",
  ".resources/smithers/README.md",
  ".resources/openclaw/README.md",
  ".resources/synara/apps/marketing/src/pages/index.astro",
  ".resources/t3code/README.md",
]);

const deepCitationTargets = new Map([
  ["hooks catalog", "/runtime/core/hooks"],
  ["skills guide", "/runtime/core/skills"],
  ["automation", "/runtime/core/automation"],
  ["sandbox profiles", "/runtime/core/sandbox/profiles"],
  ["sessions lifecycle", "/runtime/core/sessions/lifecycle"],
  ["daemon surfaces", "/runtime/core/operations/daemon"],
  ["permissions", "/runtime/core/sessions/permissions"],
]);

function listFiles(dir: string, suffix: string): string[] {
  const files: string[] = [];
  for (const entry of readdirSync(dir)) {
    const fullPath = resolve(dir, entry);
    const stat = statSync(fullPath);
    if (stat.isDirectory()) {
      if (entry !== "__tests__") files.push(...listFiles(fullPath, suffix));
      continue;
    }
    if (stat.isFile() && fullPath.endsWith(suffix)) files.push(fullPath);
  }
  return files.sort();
}

function runtimeRouteExists(route: string): boolean {
  const relativeRoute = route.replace(/^\/runtime\/?/, "");
  if (!relativeRoute) return true;
  return (
    statSync(resolve(runtimeRoot, `${relativeRoute}.mdx`), { throwIfNoEntry: false })?.isFile() ===
      true ||
    statSync(resolve(runtimeRoot, relativeRoute, "index.mdx"), {
      throwIfNoEntry: false,
    })?.isFile() === true
  );
}

describe("landing truth", () => {
  it("does not imply v0 signature verification before the runtime verifies trust proofs", () => {
    const violations = listFiles(landingRoot, ".tsx").flatMap(file => {
      const source = readFileSync(file, "utf8");
      return [...source.matchAll(/\b(?:signed|verified identity|Ed25519)\b/gi)].map(
        match => `${relative(siteRoot, file)}: ${match[0]}`
      );
    });

    expect(violations).toEqual([]);
  });

  it("keeps named market scopes attached to present, approved evidence sources", () => {
    const source = readFileSync(comparisonPath, "utf8");
    const citedPaths = [...source.matchAll(/sourcePath:\s*"([^"]+)"/g)].map(
      match => match[1] ?? ""
    );

    expect(citedPaths.length).toBeGreaterThan(0);
    expect(
      citedPaths.flatMap(sourcePath => {
        if (!approvedMarketSources.has(sourcePath)) return [`unapproved source: ${sourcePath}`];
        if (!existsSync(resolve(repoRoot, sourcePath))) return [`missing source: ${sourcePath}`];
        return [];
      })
    ).toEqual([]);
    expect(source).not.toMatch(/\b(?:none provides|has no equivalent|lacks an? equivalent)\b/i);
  });

  it("points landing source citations at the specific docs they name", () => {
    const violations = listFiles(landingRoot, ".tsx").flatMap(file => {
      const source = readFileSync(file, "utf8");
      return [...source.matchAll(/cite:\s*\{\s*href:\s*"([^"]+)",\s*label:\s*"([^"]+)"/g)]
        .map(match => ({ href: match[1] ?? "", label: match[2] ?? "" }))
        .filter(cite => deepCitationTargets.has(cite.label))
        .filter(cite => cite.href !== deepCitationTargets.get(cite.label))
        .map(cite => `${relative(siteRoot, file)}: ${cite.label} -> ${cite.href}`);
    });
    const missingTargets = [...deepCitationTargets.values()]
      .filter(route => !runtimeRouteExists(route))
      .map(route => `missing route: ${route}`);

    expect([...violations, ...missingTargets]).toEqual([]);
  });
});
