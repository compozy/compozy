import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";

import { describe, expect, it } from "vitest";

import { createExtension, parseArgs } from "../src/index.js";

const execFileAsync = promisify(execFile);

describe("@compozy/create-extension", () => {
  it("parses CLI options", () => {
    expect(parseArgs(["demo", "--template", "prompt-decorator", "--skip-install"])).toEqual({
      name: "demo",
      template: "prompt-decorator",
      skipInstall: true,
    });
  });

  it("copies the lifecycle observer template into a buildable project", async () => {
    const root = await mkdtemp(join(tmpdir(), "compozy-create-extension-"));
    await buildLocalSDK();
    const sdkSpec = `file:${resolve("sdk/extension-sdk-ts")}`;

    const result = await createExtension({
      directory: root,
      name: "my-ext",
      sdkSpec,
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
});

async function buildLocalSDK(): Promise<void> {
  await execFileAsync("npx", ["tsc", "-p", "sdk/extension-sdk-ts/tsconfig.json"], {
    cwd: process.cwd(),
    env: process.env,
  });
}
