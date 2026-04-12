import { mkdtemp, readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { execFile } from "node:child_process";
import { promisify } from "node:util";

import { describe, expect, it } from "vitest";

import { createExtension, parseArgs } from "../src/index.js";

const execFileAsync = promisify(execFile);
const repoRoot = resolve(fileURLToPath(new URL("../../..", import.meta.url)));
const createExtensionCLI = resolve(repoRoot, "sdk/create-extension/dist/bin/create-extension.js");
const localSDKSpec = pathToFileURL(resolve(repoRoot, "sdk/extension-sdk-ts")).href;
const localGoSDKReplace = repoRoot;

describe("@compozy/create-extension", () => {
  it("parses CLI options", () => {
    expect(parseArgs(["demo", "--template", "prompt-decorator", "--skip-install"])).toEqual({
      name: "demo",
      template: "prompt-decorator",
      skipInstall: true,
    });
  });

  it("emits a working CLI entrypoint", async () => {
    const root = await mkdtemp(join(tmpdir(), "compozy-create-extension-cli-"));
    await buildLocalPackages();

    const result = await execFileAsync("node", [createExtensionCLI, "cli-ext", "--skip-install"], {
      cwd: root,
      env: {
        ...process.env,
        COMPOZY_EXTENSION_SDK_SPEC: localSDKSpec,
      },
    });

    expect(result.stdout).toContain("Created lifecycle-observer extension");
    expect(result.stderr ?? "").toBe("");
    expect(await readProjectFile(root, "cli-ext", "extension.toml")).toContain('name = "cli-ext"');
  }, 120_000);

  it("copies the lifecycle observer template into a buildable project", async () => {
    const root = await mkdtemp(join(tmpdir(), "compozy-create-extension-"));
    await buildLocalPackages();

    const result = await createExtension({
      directory: root,
      name: "my-ext",
      sdkSpec: localSDKSpec,
    });

    expect(result.runtime).toBe("typescript");

    await execFileAsync("npm", ["run", "build"], {
      cwd: result.targetDir,
      env: process.env,
    });
    await execFileAsync("npm", ["test"], {
      cwd: result.targetDir,
      env: process.env,
    });
  }, 120_000);

  it("copies the review provider template into a buildable project", async () => {
    const root = await mkdtemp(join(tmpdir(), "compozy-create-extension-review-"));
    await buildLocalPackages();

    const result = await createExtension({
      directory: root,
      name: "review-ext",
      template: "review-provider",
      sdkSpec: localSDKSpec,
    });

    expect(result.runtime).toBe("typescript");

    await execFileAsync("npm", ["run", "build"], {
      cwd: result.targetDir,
      env: process.env,
    });
    await execFileAsync("npm", ["test"], {
      cwd: result.targetDir,
      env: process.env,
    });
  }, 120_000);

  it("scaffolds a Go project against the local repository when requested", async () => {
    const root = await mkdtemp(join(tmpdir(), "compozy-create-extension-go-"));
    await buildLocalPackages();

    const result = await createExtension({
      directory: root,
      name: "go-ext",
      runtime: "go",
      template: "prompt-decorator",
      moduleName: "example.com/go-ext",
      goSDKReplace: localGoSDKReplace,
    });

    const goMod = await readProjectFile(root, "go-ext", "go.mod");
    expect(result.runtime).toBe("go");
    expect(goMod).toContain("module example.com/go-ext");
    expect(goMod).toContain("replace github.com/compozy/compozy =>");

    await execFileAsync("go", ["test", "./..."], {
      cwd: result.targetDir,
      env: process.env,
    });
  }, 120_000);
});

async function buildLocalPackages(): Promise<void> {
  await execFileAsync(
    "npm",
    [
      "run",
      "build",
      "--workspace",
      "@compozy/extension-sdk",
      "--workspace",
      "@compozy/create-extension",
    ],
    {
      cwd: repoRoot,
      env: process.env,
    }
  );
}

async function readProjectFile(root: string, project: string, file: string): Promise<string> {
  return readFile(join(root, project, file), "utf8");
}
