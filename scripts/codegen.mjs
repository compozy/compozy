#!/usr/bin/env node
/**
 * Minimal deterministic codegen for the daemon web UI.
 * Regenerates `web/src/generated/compozy-openapi.d.ts` from
 * `openapi/compozy-daemon.json` via `openapi-typescript`.
 *
 * Usage: node scripts/codegen.mjs [--check]
 *  - no flag: regenerate the file in place
 *  - --check: regenerate into a temp file and fail if it differs
 */

import { spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const mode = process.argv.includes("--check") ? "check" : "write";

const here = fileURLToPath(new URL(".", import.meta.url));
const repoRoot = resolve(here, "..");
const source = resolve(repoRoot, "openapi/compozy-daemon.json");
const target = resolve(repoRoot, "web/src/generated/compozy-openapi.d.ts");

function resolveCli() {
  const candidates = [
    resolve(repoRoot, "node_modules/openapi-typescript/bin/cli.js"),
    resolve(repoRoot, "web/node_modules/openapi-typescript/bin/cli.js"),
  ];
  for (const candidate of candidates) {
    if (existsSync(candidate)) {
      return candidate;
    }
  }
  console.error(
    "codegen: openapi-typescript CLI not found; run `bun install` before running codegen."
  );
  process.exit(1);
}

function run(output) {
  const cli = resolveCli();
  const args = [cli, source, "-o", output];
  const result = spawnSync(process.execPath, args, {
    cwd: repoRoot,
    stdio: ["ignore", "inherit", "inherit"],
  });
  if (result.status !== 0) {
    const code = typeof result.status === "number" ? result.status : 1;
    process.exit(code);
  }
}

if (mode === "write") {
  run(target);
  process.exit(0);
}

const workDir = mkdtempSync(join(tmpdir(), "compozy-codegen-"));
try {
  const candidate = join(workDir, "compozy-openapi.d.ts");
  run(candidate);
  const next = readFileSync(candidate, "utf8");
  let current = "";
  try {
    current = readFileSync(target, "utf8");
  } catch {
    current = "";
  }
  if (next !== current) {
    writeFileSync(target, next);
    console.error(
      [
        "codegen-check: web/src/generated/compozy-openapi.d.ts is out of date.",
        "The file was regenerated — re-run the build and commit the change.",
      ].join("\n")
    );
    process.exit(1);
  }
} finally {
  rmSync(workDir, { recursive: true, force: true });
}
