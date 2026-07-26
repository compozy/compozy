// Suite: AGH Web data-boundary lint
// Invariant: Server-query consumers reuse canonical option and key factories.
// Boundary IN: Oxlint parsing and compozy-data-boundaries diagnostics.
// Boundary OUT: Query/cache behavior, owned by Web system and route suites.

import { spawnSync } from "node:child_process";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";
import plugin from "../data-boundaries.mjs";

const TEST_DIR = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(TEST_DIR, "../..");
const PLUGIN_PATH = join(REPO_ROOT, "lint-plugins/data-boundaries.mjs");
const RULES = {
  "compozy-data-boundaries/no-inline-query-key": "error",
  "compozy-data-boundaries/require-canonical-query-options": "error",
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

async function runOxlint({ filename, source }) {
  const root = await mkdtemp(join(tmpdir(), "agh-data-boundaries-"));
  tempRoots.push(root);
  const sourcePath = join(root, filename);
  await mkdir(dirname(sourcePath), { recursive: true });
  await writeFile(sourcePath, source, "utf8");
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
      sourcePath,
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

describe("compozy-data-boundaries lint plugin", () => {
  it("Should register the canonical options and query-key rules", () => {
    expect(Object.keys(plugin.rules).sort()).toEqual([
      "no-inline-query-key",
      "require-canonical-query-options",
    ]);
  });

  it("Should reject a hook that defines its server query inline", async () => {
    const result = await runOxlint({
      filename: "web/src/systems/tasks/hooks/use-tasks.ts",
      source:
        'import { useQuery as useServerQuery } from "@tanstack/react-query";\n' +
        'export function useTasks() { return useServerQuery({ queryKey: ["tasks"], queryFn: fetchTasks }); }\n',
    });
    expect(result.exitCode).not.toBe(0);
    expect(result.messages.join("\n")).toContain("Move queryKey/queryFn");
    expect(result.messages.join("\n")).toContain("named query-key factory");
  });

  it("Should allow a canonical option factory with consumer-only overrides", async () => {
    const result = await runOxlint({
      filename: "web/src/systems/status/hooks/use-daemon-status.ts",
      source:
        'import { useQuery } from "@tanstack/react-query";\n' +
        "export function useStatus(enabled) { return useQuery({ ...statusOptions(), enabled, select: selectDaemon }); }\n",
    });
    expect(result.exitCode).toBe(0);
    expect(result.messages).toEqual([]);
  });

  it("Should reject an inline query-key array in a cache writer", async () => {
    const result = await runOxlint({
      filename: "web/src/systems/tasks/hooks/use-task-stream.ts",
      source:
        'export function refresh(queryClient) { return queryClient.invalidateQueries({ queryKey: ["tasks", "list"] }); }\n',
    });
    expect(result.exitCode).not.toBe(0);
    expect(result.messages.join("\n")).toContain("named query-key factory");
  });

  it("Should allow named key factories and non-consumer definitions", async () => {
    const inputs = [
      {
        filename: "web/src/systems/tasks/hooks/use-task-stream.ts",
        source:
          "export function refresh(queryClient) { return queryClient.invalidateQueries({ queryKey: tasksKeys.lists() }); }\n",
      },
      {
        filename: "web/src/systems/tasks/lib/query-options.ts",
        source:
          'export const options = queryOptions({ queryKey: ["tasks"], queryFn: fetchTasks });\n',
      },
      {
        filename: "web/src/systems/tasks/hooks/__tests__/use-tasks.test.tsx",
        source: 'export const query = useQuery({ queryKey: ["tasks"], queryFn: fetchTasks });\n',
      },
    ];
    for (const input of inputs) {
      const result = await runOxlint(input);
      expect(result.exitCode, input.filename).toBe(0);
      expect(result.messages, input.filename).toEqual([]);
    }
  });
});
