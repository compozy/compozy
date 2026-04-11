import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";

import { describe, expect, it } from "vitest";

import { createExtension, TEMPLATE_NAMES } from "../../create-extension/src/index.js";

const execFileAsync = promisify(execFile);

describe("starter templates", () => {
  for (const template of TEMPLATE_NAMES) {
    it(`passes its local test contract: ${template}`, async () => {
      const root = await mkdtemp(join(tmpdir(), `compozy-template-${template}-`));
      await buildLocalSDK();
      const sdkSpec = `file:${resolve("sdk/extension-sdk-ts")}`;

      const result = await createExtension({
        directory: root,
        name: `${template}-sample`,
        sdkSpec,
        template,
      });

      const testResult = await execFileAsync("npm", ["test"], {
        cwd: result.targetDir,
        env: process.env,
      });
      expect(testResult.stderr ?? "").not.toContain("not ok");
    }, 120_000);
  }
});

async function buildLocalSDK(): Promise<void> {
  await execFileAsync("npx", ["tsc", "-p", "sdk/extension-sdk-ts/tsconfig.json"], {
    cwd: process.cwd(),
    env: process.env,
  });
}
