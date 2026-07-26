import { join, resolve } from "node:path";
import { mkdir, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";

import { beforeEach, describe, expect, it } from "vitest";

import { Extension } from "../../extension.js";
import { TestHarness } from "../harness.js";

const sourceDir = resolve(__dirname, "..");
const packageDir = resolve(sourceDir, "../..");

describe("TestHarness", () => {
  let harness: TestHarness;

  beforeEach(() => {
    harness = new TestHarness();
  });

  it("mockHostAPI returns mocked responses", async () => {
    const sessions: string[] = [];
    harness.mockHostAPI("sessions/list", async () => [
      {
        id: "sess-1",
        agent: "claude",
        state: "active",
        created_at: "2026-04-10T12:00:00.000Z",
      },
    ]);

    const extension = new Extension({
      name: "mock-host",
      version: "0.1.0",
      actions: { requires: ["sessions/list"] },
    });
    extension.onReady(async host => {
      const result = await host.sessions.list();
      sessions.push(result[0]!.id);
    });

    await harness.loadExtension(extension);
    expect(sessions).toEqual(["sess-1"]);
  });

  it("loadExtension loads an extension without spawning a subprocess", async () => {
    const extension = new Extension({
      name: "inline",
      version: "0.1.0",
    });

    await expect(harness.loadExtension(extension)).resolves.toBe(extension);
  });

  it("call invokes extension handlers directly", async () => {
    const extension = new Extension({
      name: "caller",
      version: "0.1.0",
    });

    extension.handle("memory/store", async (_ctx, params: { key: string; content: string }) => ({
      key: params.key,
      content: params.content,
    }));

    await harness.loadExtension(extension);
    await expect(harness.call("memory/store", { key: "x", content: "y" })).resolves.toEqual({
      key: "x",
      content: "y",
    });
  });

  it("loads a module export by path", async () => {
    const dir = join(tmpdir(), `agh-harness-${Date.now()}`);
    const filePath = join(dir, "extension.ts");
    await mkdir(dir, { recursive: true });
    await writeFile(
      filePath,
      `import { Extension } from ${JSON.stringify(resolve(packageDir, "src/index.ts"))};
       export function createExtension(options = {}) {
         const extension = new Extension({ name: "path-ext", version: "0.1.0" }, options);
         extension.handle("health_check", async () => ({ healthy: true }));
         return extension;
       }`
    );

    await expect(harness.loadExtension(filePath)).resolves.toBeInstanceOf(Extension);
  });
});
