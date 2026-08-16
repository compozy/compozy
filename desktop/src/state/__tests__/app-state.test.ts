import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { AppStatePublisher } from "../app-state";

// Invariant: every published app.json is a complete schema-v2 snapshot of shell and update truth.
describe("app state publisher", () => {
  it("Should publish product and operation state through one atomic record", async () => {
    const home = await mkdtemp(join(tmpdir(), "compozy-app-state-"));
    const path = join(home, "app.json");
    try {
      const publisher = new AppStatePublisher({
        path,
        appVersion: "0.3.0",
        channel: "development",
        startedAt: new Date("2026-08-16T00:00:00Z"),
      });
      await publisher.setRuntime("0.3.0", true);
      await publisher.setOperation({
        schema_version: 1,
        operation_id: "operation-1",
        revision: 8,
        targets: ["app"],
        active_target: "app",
        percent: 42,
        app: {
          from_version: "0.3.0",
          to_version: "0.3.1",
          release_tag: "v0.3.1",
          asset: "CompozyOS-0.3.1-arm64.zip",
          digest: "sha256:abc",
          attempt_id: "attempt-1",
          phase: "applying",
          consecutive_failures: 0,
        },
        holder: null,
        waiting: "",
      });
      await publisher.publish({ state: "product", origin: "http://127.0.0.1:2123", owned: true });

      const record = JSON.parse(await readFile(path, "utf8")) as Record<string, unknown>;
      expect(record).toMatchObject({
        schema_version: 2,
        state: "product",
        origin: "http://127.0.0.1:2123",
        owned: true,
        update: {
          app_state: "applying",
          runtime_state: "idle",
          operation_id: "operation-1",
          phase: "download",
          percent: 42,
        },
      });
    } finally {
      await rm(home, { recursive: true, force: true });
    }
  });
});
