// Suite: Compozy Web computer-scoped query-key lint (UT-092)
// Invariant: TanStack Query keys in web/src/systems/* include the leading ["computer", id] scope.
// Boundary IN: Oxlint parsing and compozy/query-key-computer-scope diagnostics.
// Boundary OUT: Runtime cache eviction, owned by web/src/systems/computers tests.

import { spawnSync } from "node:child_process";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";
import plugin from "../computer-scope.mjs";

const TEST_DIR = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(TEST_DIR, "../..");
const PLUGIN_PATH = join(REPO_ROOT, "lint-plugins/computer-scope.mjs");
const RULES = {
  "compozy/query-key-computer-scope": "error",
};
const tempRoots = [];

afterEach(async () => {
  const roots = tempRoots.splice(0);
  await Promise.all(roots.map(root => rm(root, { recursive: true, force: true })));
});

function pluginReferenceFrom(root) {
  const reference = relative(root, PLUGIN_PATH).replaceAll("\\", "/");
  return reference.startsWith(".") ? reference : `./${reference}`;
}

function collectMessages(value) {
  if (!value) return [];
  if (Array.isArray(value)) return value.flatMap(item => collectMessages(item));
  if (typeof value !== "object") return [];
  const messages = [];
  if (typeof value.message === "string") messages.push(value.message);
  if (value.diagnostic && typeof value.diagnostic.message === "string") {
    messages.push(value.diagnostic.message);
  }
  for (const key of ["diagnostics", "errors", "warnings", "files"]) {
    if (Array.isArray(value[key])) messages.push(...collectMessages(value[key]));
  }
  return messages;
}

async function runOxlint(input) {
  const inputs = Array.isArray(input) ? input : [input];
  const root = await mkdtemp(join(tmpdir(), "compozy-computer-scope-"));
  tempRoots.push(root);
  const sourcePaths = [];
  for (const { filename, source } of inputs) {
    const sourcePath = join(root, filename);
    await mkdir(dirname(sourcePath), { recursive: true });
    await writeFile(sourcePath, source, "utf8");
    sourcePaths.push(sourcePath);
  }
  const configPath = join(root, ".oxlintrc.json");
  await writeFile(
    configPath,
    JSON.stringify({ plugins: [], jsPlugins: [pluginReferenceFrom(root)], rules: RULES }),
    "utf8"
  );

  const result = spawnSync(
    "bunx",
    [
      "oxlint",
      "--config",
      configPath,
      "--format",
      "json",
      "--no-ignore",
      "--threads",
      "1",
      ...sourcePaths,
    ],
    {
      cwd: root,
      encoding: "utf8",
      env: { ...process.env, FORCE_COLOR: "0", NO_COLOR: "1" },
    }
  );
  if (result.error) throw result.error;
  const stdout = result.stdout.trim();
  return {
    exitCode: result.status,
    messages: stdout ? collectMessages(JSON.parse(stdout)) : [],
  };
}

describe("compozy/query-key-computer-scope lint rule (UT-092)", () => {
  it("Should register the query-key-computer-scope rule under compozy", () => {
    expect(plugin.meta.name).toBe("compozy");
    expect(Object.keys(plugin.rules)).toEqual(["query-key-computer-scope"]);
  });

  it("UT-092: Should flag useQuery({ queryKey: ['sessions'] }) inside web/src/systems/*/hooks", async () => {
    const result = await runOxlint({
      filename: "web/src/systems/session/hooks/use-sessions-list.ts",
      source:
        'import { useQuery } from "@tanstack/react-query";\n' +
        'export function useSessionsList() { return useQuery({ queryKey: ["sessions"], queryFn: fetchSessions }); }\n',
    });
    expect(result.exitCode).not.toBe(0);
    expect(result.messages.join("\n")).toContain("computer-scoped");
  });

  it("UT-092: Should pass useQuery({ queryKey: ['computer', id, 'sessions'] }) inside web/src/systems/*/hooks", async () => {
    const result = await runOxlint({
      filename: "web/src/systems/session/hooks/use-sessions-list.ts",
      source:
        'import { useQuery } from "@tanstack/react-query";\n' +
        'export function useSessionsList(id: string) { return useQuery({ queryKey: ["computer", id, "sessions"], queryFn: fetchSessions }); }\n',
    });
    expect(result.exitCode).toBe(0);
    expect(result.messages).toEqual([]);
  });

  it("Should allow spread of computer-scoped key factories and controller-only systems", async () => {
    const result = await runOxlint([
      {
        filename: "web/src/systems/session/lib/query-options.ts",
        source:
          'import { queryOptions } from "@tanstack/react-query";\n' +
          "export const opts = (id: string) => queryOptions({ queryKey: [...sessionKeys.all, id], queryFn: fetchSessions });\n",
      },
      {
        filename: "web/src/systems/computers/hooks/use-computers.ts",
        source:
          'import { useQuery } from "@tanstack/react-query";\n' +
          'export function useComputers() { return useQuery({ queryKey: ["computers"], queryFn: fetchComputers }); }\n',
      },
      {
        filename: "web/src/systems/account/hooks/use-account.ts",
        source:
          'import { useQuery } from "@tanstack/react-query";\n' +
          'export function useAccount() { return useQuery({ queryKey: ["account"], queryFn: fetchAccount }); }\n',
      },
    ]);
    expect(result.exitCode).toBe(0);
    expect(result.messages).toEqual([]);
  });
});
