import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { SUPPORTED_AGENT_PROVIDERS } from "@/components/landing/provider-data";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const repoRoot = resolve(siteRoot, "../..");
const landingRoot = resolve(siteRoot, "components/landing");
const runtimeRoot = resolve(siteRoot, "content/runtime");
const providerSourceRoot = resolve(repoRoot, "internal/config");
const serverLandingMetricModules = ["hero.tsx", "comparison.tsx"].map(file =>
  resolve(landingRoot, file)
);
const clientSupportedAgentsImportPattern = /(?:from\s+|import\()\s*["'][^"']*supported-agents["']/;

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
      if (entry === "__tests__") {
        continue;
      }
      files.push(...listFiles(fullPath, suffix));
      continue;
    }
    if (stat.isFile() && fullPath.endsWith(suffix)) {
      files.push(fullPath);
    }
  }
  return files.sort();
}

function simpleGoStringConstants(source: string): Map<string, string> {
  const constants = new Map<string, string>();
  const parseLine = (line: string) => {
    const match = line.match(/^\s*([A-Za-z_]\w*)(?:\s+[A-Za-z_]\w*)?\s*=\s*"([^"]*)"\s*$/);
    if (match?.[1] && match[2] !== undefined) {
      constants.set(match[1], match[2]);
    }
  };

  for (const group of source.matchAll(/const\s*\(([\s\S]*?)\n\)/g)) {
    group[1]?.split("\n").forEach(parseLine);
  }
  for (const match of source.matchAll(
    /const\s+([A-Za-z_]\w*)(?:\s+[A-Za-z_]\w*)?\s*=\s*"([^"]*)"/g
  )) {
    constants.set(match[1] ?? "", match[2] ?? "");
  }

  return constants;
}

function goPackageSource(dir: string): string {
  return readdirSync(dir)
    .filter(file => file.endsWith(".go") && !file.endsWith("_test.go"))
    .sort()
    .map(file => readFileSync(resolve(dir, file), "utf8"))
    .join("\n");
}

function resolveGoString(token: string, constants: Map<string, string>): string | undefined {
  if (token.startsWith('"')) {
    return token.slice(1, -1);
  }
  return constants.get(token);
}

function mustResolveGoString(token: string, constants: Map<string, string>, field: string): string {
  const resolved = resolveGoString(token, constants);
  if (resolved === undefined) {
    throw new Error(`unresolved builtin provider ${field}: ${token}`);
  }
  return resolved;
}

function builtinProviderNamesFromSource(source: string): Map<string, string> {
  const mapSource =
    source.match(/var builtinProviders = map\[string\]ProviderConfig\{([\s\S]*?)\n\}/)?.[1] ?? "";
  const constants = simpleGoStringConstants(source);
  const providers = new Map<string, string>();
  for (const match of mapSource.matchAll(
    /("[^"]+"|[A-Za-z_]\w*):\s*\{[\s\S]*?DisplayName:\s*("[^"]+"|[A-Za-z_]\w*)/g
  )) {
    const id = mustResolveGoString(match[1] ?? "", constants, "id");
    const name = mustResolveGoString(match[2] ?? "", constants, "display name");
    providers.set(id, name);
  }
  if (providers.size === 0) {
    throw new Error("no builtin providers parsed from runtime registry");
  }
  return providers;
}

function builtinProviderNames(): Map<string, string> {
  return builtinProviderNamesFromSource(goPackageSource(providerSourceRoot));
}

function runtimeRouteExists(route: string): boolean {
  const relativeRoute = route.replace(/^\/runtime\/?/, "");
  if (!relativeRoute) {
    return true;
  }
  return (
    statSync(resolve(runtimeRoot, `${relativeRoute}.mdx`), { throwIfNoEntry: false })?.isFile() ===
      true ||
    statSync(resolve(runtimeRoot, relativeRoute, "index.mdx"), {
      throwIfNoEntry: false,
    })?.isFile() === true
  );
}

describe("landing truth", () => {
  it("keeps provider names aligned with the runtime built-in registry", () => {
    const runtimeProviders = builtinProviderNames();
    const landingProviders = new Map(
      SUPPORTED_AGENT_PROVIDERS.map(provider => [provider.id, provider.name])
    );

    expect(Object.fromEntries(landingProviders)).toEqual(Object.fromEntries(runtimeProviders));
  });

  it("keeps server-rendered landing metrics off client-only provider UI modules", () => {
    const violations = serverLandingMetricModules.flatMap(file => {
      const source = readFileSync(file, "utf8");
      if (!clientSupportedAgentsImportPattern.test(source)) {
        return [];
      }
      return [
        `${relative(siteRoot, file)} imports the client-only supported agent UI module for a server metric`,
      ];
    });

    expect(violations).toEqual([]);
  });

  it("fails loudly when runtime provider constants cannot be resolved", () => {
    const source = `
const providerClaudeKey = "claude"

var builtinProviders = map[string]ProviderConfig{
	providerClaudeKey: {
		DisplayName: unresolvedDisplayName,
	},
}
`;

    expect(() => builtinProviderNamesFromSource(source)).toThrow(
      "unresolved builtin provider display name: unresolvedDisplayName"
    );
  });

  it("does not imply v0 signature verification before the runtime verifies trust proofs", () => {
    const violations = listFiles(landingRoot, ".tsx").flatMap(file => {
      const source = readFileSync(file, "utf8");
      return [...source.matchAll(/\b(?:signed|verified identity|Ed25519)\b/gi)].map(
        match => `${relative(siteRoot, file)}: ${match[0]}`
      );
    });

    expect(violations).toEqual([]);
  });

  it("points landing source citations at the specific docs they name", () => {
    const violations = listFiles(landingRoot, ".tsx").flatMap(file => {
      const source = readFileSync(file, "utf8");
      return [...source.matchAll(/cite:\s*\{\s*href:\s*"([^"]+)",\s*label:\s*"([^"]+)"/g)]
        .map(match => ({
          href: match[1] ?? "",
          label: match[2] ?? "",
        }))
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
