import { createHash } from "node:crypto";
import { chmod, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import type { BootstrapEvent } from "../bootstrap-event";
import { BootstrapRunner } from "../bootstrap-runner";

// Invariant: every bootstrap event the runtime emits is persisted to desktop-bootstrap.jsonl with
// its diagnostic fields. Owner: desktop bootstrap runner. Canonical suite: bootstrap-runner.test.ts.
describe("BootstrapRunner", () => {
  it.skipIf(process.platform === "win32")(
    "Should persist data-migration progress emitted by the runtime",
    async () => {
      const directory = await mkdtemp(join(tmpdir(), "compozy-desktop-bootstrap-runner-"));
      const bundle = join(directory, "compozy");
      const manifest = join(directory, "runtime-manifest.json");
      const logPath = join(directory, "logs", "desktop-bootstrap.jsonl");
      const migrating = {
        type: "bootstrap",
        phase: "provision",
        status: "started",
        message: "Upgrading CompozyOS data (0%).",
        resolution: "provision",
        stage: "migrate",
        pct: 0,
      };
      const ready = {
        type: "bootstrap",
        phase: "ready",
        status: "completed",
        message: "Ready",
        resolution: "provision",
        daemon: { origin: "http://127.0.0.1:2123", version: "1.0.0", pid: 42, owned: true },
      };
      const script = `#!/bin/sh\nprintf '%s\\n' '${JSON.stringify(migrating)}' '${JSON.stringify(ready)}'\n`;
      try {
        await writeFile(bundle, script);
        await chmod(bundle, 0o700);
        await writeFile(
          manifest,
          JSON.stringify({
            schema_version: 1,
            asset: "compozy",
            sha256: createHash("sha256").update(script).digest("hex"),
          })
        );
        const runner = new BootstrapRunner({
          bundlePath: bundle,
          manifestPath: manifest,
          logPath,
          minimumRuntime: ">=1.0.0",
          appVersion: "1.0.0",
          channel: "development",
          bootId: "boot-runner",
        });
        const observed: BootstrapEvent[] = [];
        await expect(
          runner.run(async event => {
            observed.push(event);
          })
        ).resolves.toMatchObject({ pid: 42, resolution: "provision" });
        expect(observed[0]).toMatchObject({ stage: "migrate", pct: 0 });
        const records = (await readFile(logPath, "utf8"))
          .trim()
          .split("\n")
          .map(line => JSON.parse(line) as Record<string, unknown>);
        expect(records[0]).toMatchObject({
          boot_id: "boot-runner",
          level: "INFO",
          phase: "provision",
          status: "started",
          resolution: "provision",
          stage: "migrate",
          pct: 0,
        });
        expect(records[1]).not.toHaveProperty("stage");
      } finally {
        await rm(directory, { recursive: true });
      }
    }
  );
});
