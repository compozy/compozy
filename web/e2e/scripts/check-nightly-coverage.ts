import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

import {
  e2eScenarioContracts,
  validateNightlySpecCoverage,
  validateScenarioContracts,
} from "../fixtures/scenario-contracts";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const e2eRoot = path.resolve(scriptDir, "..");
const repoRoot = path.resolve(e2eRoot, "..", "..");
const testsDir = path.join(e2eRoot, "__tests__");

async function main(): Promise<void> {
  const contractErrors = validateScenarioContracts(e2eScenarioContracts);
  if (contractErrors.length > 0) {
    throw new Error(`nightly scenario contract is invalid:\n${contractErrors.join("\n")}`);
  }

  const specs = await readSpecTexts(testsDir);
  const coverageErrors = validateNightlySpecCoverage(e2eScenarioContracts, specs);
  if (coverageErrors.length > 0) {
    throw new Error(`nightly browser coverage is absent:\n${coverageErrors.join("\n")}`);
  }
}

async function readSpecTexts(dir: string): Promise<Record<string, string>> {
  const output: Record<string, string> = {};
  const entries = await readdir(dir, { withFileTypes: true });
  for (const entry of entries) {
    const absolutePath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      Object.assign(output, await readSpecTexts(absolutePath));
      continue;
    }
    if (!entry.name.endsWith(".spec.ts")) {
      continue;
    }
    const relativePath = path.relative(repoRoot, absolutePath).replaceAll(path.sep, "/");
    output[relativePath] = await readFile(absolutePath, "utf8");
  }
  return output;
}

main().catch(error => {
  const message = error instanceof Error ? error.message : String(error);
  console.error(message);
  process.exitCode = 1;
});
