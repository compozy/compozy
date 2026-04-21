import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const codegenScript = readFileSync(path.join(repoRoot, "scripts/codegen.mjs"), "utf8");

function createCodegenFixture(initialOutput: string) {
  const root = mkdtempSync(path.join(tmpdir(), "compozy-codegen-script-"));
  const scriptPath = path.join(root, "scripts/codegen.mjs");
  const cliPath = path.join(root, "node_modules/openapi-typescript/bin/cli.js");
  const sourcePath = path.join(root, "openapi/compozy-daemon.json");
  const targetPath = path.join(root, "web/src/generated/compozy-openapi.d.ts");

  mkdirSync(path.dirname(scriptPath), { recursive: true });
  mkdirSync(path.dirname(cliPath), { recursive: true });
  mkdirSync(path.dirname(sourcePath), { recursive: true });
  mkdirSync(path.dirname(targetPath), { recursive: true });

  writeFileSync(scriptPath, codegenScript, "utf8");
  writeFileSync(
    cliPath,
    [
      "const { mkdirSync, readFileSync, writeFileSync } = require('node:fs');",
      "const { dirname } = require('node:path');",
      "const [, , input, flag, output] = process.argv;",
      "if (flag !== '-o' || !output) process.exit(2);",
      "const spec = JSON.parse(readFileSync(input, 'utf8'));",
      "mkdirSync(dirname(output), { recursive: true });",
      "writeFileSync(output, `// generated ${spec.info.version}\\n`, 'utf8');",
    ].join("\n"),
    "utf8"
  );
  writeFileSync(sourcePath, JSON.stringify({ info: { version: "2.0.0" } }), "utf8");
  writeFileSync(targetPath, initialOutput, "utf8");

  return { root, scriptPath, targetPath };
}

describe("scripts/codegen.mjs", () => {
  it("--check reports drift without mutating the tracked output", () => {
    const fixture = createCodegenFixture("// stale output\n");

    const result = spawnSync(process.execPath, [fixture.scriptPath, "--check"], {
      cwd: fixture.root,
      encoding: "utf8",
    });

    expect(result.status).toBe(1);
    expect(result.stderr).toContain(
      "codegen-check: web/src/generated/compozy-openapi.d.ts is out of date."
    );
    expect(readFileSync(fixture.targetPath, "utf8")).toBe("// stale output\n");
  });

  it("regenerates the tracked output in write mode", () => {
    const fixture = createCodegenFixture("// stale output\n");

    const result = spawnSync(process.execPath, [fixture.scriptPath], {
      cwd: fixture.root,
      encoding: "utf8",
    });

    expect(result.status).toBe(0);
    expect(readFileSync(fixture.targetPath, "utf8")).toBe("// generated 2.0.0\n");
  });
});
