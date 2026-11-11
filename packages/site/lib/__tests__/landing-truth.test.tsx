import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { HREF } from "@/components/landing/content";
import { BUILTIN_PROVIDER_INTEGRATIONS } from "@/components/landing/provider-data";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const repoRoot = resolve(siteRoot, "../..");
const landingRoot = resolve(siteRoot, "components/landing");
const runtimeRoot = resolve(siteRoot, "content/docs");
const providerSourceRoot = resolve(repoRoot, "internal/config");
const USE_CLIENT = /^\s*["']use client["'];/;

/** A `@/` or relative import, resolved to its source file; `null` for packages. */
function resolveSiteImport(from: string, specifier: string): string | null {
  const base = specifier.startsWith("@/")
    ? resolve(siteRoot, specifier.slice(2))
    : specifier.startsWith(".")
      ? resolve(dirname(from), specifier)
      : null;
  if (!base) return null;
  for (const candidate of [`${base}.tsx`, `${base}.ts`, resolve(base, "index.ts")]) {
    if (statSync(candidate, { throwIfNoEntry: false })?.isFile()) return candidate;
  }
  return null;
}

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
  if (token.startsWith('"')) return token.slice(1, -1);
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
  const relativeRoute = route.replace(/^\/docs\/?/, "");
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
  it("keeps provider names aligned with the runtime built-in registry", () => {
    const runtimeProviders = builtinProviderNames();
    const landingProviders = new Map(
      BUILTIN_PROVIDER_INTEGRATIONS.map(provider => [provider.id, provider.name])
    );

    expect(Object.fromEntries(landingProviders)).toEqual(Object.fromEntries(runtimeProviders));
  });

  it("keeps server-rendered landing values off client modules, which only export components to the server", () => {
    // A server component that imports a constant (the provider count) from a "use client" module
    // receives a client reference, not the value, so it would render a broken metric.
    const violations = listFiles(landingRoot, ".tsx")
      .filter(file => !USE_CLIENT.test(readFileSync(file, "utf8")))
      .flatMap(file => {
        const source = readFileSync(file, "utf8");
        return [
          ...source.matchAll(/import\s+(?!type\b)\{([^}]*)\}\s+from\s+["']([^"']+)["']/g),
        ].flatMap(([, names = "", specifier = ""]) => {
          const target = resolveSiteImport(file, specifier);
          if (!target || !USE_CLIENT.test(readFileSync(target, "utf8"))) return [];
          return names
            .split(",")
            .map(name => name.trim().split(/\s+as\s+/)[0] ?? "")
            .filter(name => name && !name.startsWith("type ") && !/^[A-Z][a-z]/.test(name))
            .map(name => `${relative(siteRoot, file)} imports ${name} from client ${specifier}`);
        });
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

  it("points every landing docs link at a published docs page", () => {
    const docsLinks = Object.values(HREF).filter(href => href.startsWith("/docs"));
    expect(docsLinks.length).toBeGreaterThan(0);
    expect(docsLinks.filter(route => !runtimeRouteExists(route))).toEqual([]);
  });
});
