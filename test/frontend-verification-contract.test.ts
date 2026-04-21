import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import {
  PLAYWRIGHT_WORKFLOW_SLUGS,
  resolvePlaywrightPaths,
} from "../web/e2e/support/daemon-fixture";

const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

interface PackageJSON {
  scripts?: Record<string, string>;
  packageManager?: string;
}

function targetDependencies(makefile: string, target: string): string[] {
  const match = makefile.match(new RegExp(`^${target}:([^\\n]*)$`, "m"));
  if (!match) {
    throw new Error(`target ${target} not found`);
  }
  return match[1].trim().split(/\s+/).filter(Boolean);
}

describe("frontend verification contract", () => {
  it("pins Bun and exposes explicit frontend verification scripts", async () => {
    const packageJSON = JSON.parse(
      await readFile(path.join(rootDir, "package.json"), "utf8")
    ) as PackageJSON;
    const bunVersion = (await readFile(path.join(rootDir, ".bun-version"), "utf8")).trim();

    expect(packageJSON.packageManager).toBe(`bun@${bunVersion}`);
    expect(packageJSON.scripts).toMatchObject({
      "frontend:bootstrap": "bun ci",
      "frontend:lint": "bun run lint",
      "frontend:typecheck": "bun run typecheck",
      "frontend:e2e": "bun run --cwd web test:e2e",
      test: "bun run test:config",
    });
    expect(packageJSON.scripts?.["frontend:test"]).toContain("packages/ui");
    expect(packageJSON.scripts?.["frontend:test"]).toContain("web test");
  });

  it("runs frontend verification before go verification and keeps build frontend-aware", async () => {
    const makefile = await readFile(path.join(rootDir, "Makefile"), "utf8");

    expect(targetDependencies(makefile, "build")).toEqual(["frontend-build", "go-build"]);
    expect(targetDependencies(makefile, "frontend-verify")).toEqual([
      "frontend-lint",
      "frontend-typecheck",
      "frontend-test",
      "frontend-build",
    ]);
    expect(targetDependencies(makefile, "verify")).toEqual([
      "frontend-verify",
      "fmt",
      "lint",
      "test",
      "go-build",
      "frontend-e2e",
    ]);
  });

  it("updates CI filters and steps for frontend, contract, and browser verification surfaces", async () => {
    const ci = await readFile(path.join(rootDir, ".github/workflows/ci.yml"), "utf8");

    for (const needle of [
      "outputs:\n      verify:",
      "verify:",
      "packages/ui/**",
      "web/**",
      "openapi/**",
      "scripts/**",
      ".bun-version",
      ".github/actions/setup-bun/**",
      "uses: ./.github/actions/setup-bun",
      "working-directory: web",
      "bunx playwright install --with-deps chromium",
      "run: make verify",
    ]) {
      expect(ci).toContain(needle);
    }
  });

  it("targets the daemon-served embedded topology instead of a dev server", async () => {
    const playwrightConfig = await readFile(path.join(rootDir, "web/playwright.config.ts"), "utf8");
    const paths = resolvePlaywrightPaths();

    expect(playwrightConfig).toContain('testDir: "./e2e"');
    expect(playwrightConfig).toContain("globalSetup:");
    expect(playwrightConfig).toContain("globalTeardown:");
    expect(playwrightConfig).not.toContain("webServer:");
    expect(paths.binaryPath).toBe(path.join(rootDir, "bin", "compozy"));
    expect(PLAYWRIGHT_WORKFLOW_SLUGS).toEqual(["daemon", "daemon-web-ui"]);
  });
});
