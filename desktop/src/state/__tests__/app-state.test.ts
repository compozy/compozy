import { access, mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { AppStatePublisher, projectAppUpdateState } from "../app-state";
import type {
  AppOperationPhase,
  RuntimeOperationPhase,
  UpdateOperation,
} from "../../update/operation-contract";

// Invariant: every published app.json is a complete schema-v2 snapshot of shell and update truth.
// Owner: desktop app-state writer. Canonical suite: app-state.test.ts.
describe("app state publisher", () => {
  it.each([
    ["pending", "accepted", undefined],
    ["applying", "applying", "download"],
    ["installer-handoff", "applying", "install"],
    ["restarted", "applying", "start"],
    ["verified", "applying", "ready"],
    ["failed", "failed", undefined],
  ] as const)("Should project app phase %s", (phase, state, uiPhase) => {
    const operation: UpdateOperation = {
      schema_version: 1,
      operation_id: "app-operation",
      revision: 1,
      targets: ["app"],
      active_target: "app",
      percent: phase === "applying" ? 25 : 100,
      app: {
        from_version: "1.0.0",
        to_version: "1.1.0",
        release_tag: "v1.1.0",
        asset: "app.zip",
        digest: "sha256:app",
        attempt_id: "attempt-1",
        phase: phase as AppOperationPhase,
        consecutive_failures: 0,
      },
      holder: null,
      waiting: "",
    };
    expect(projectAppUpdateState(operation)).toMatchObject({
      app_state: state,
      runtime_state: "idle",
      ...(uiPhase ? { phase: uiPhase } : {}),
    });
  });

  it.each([
    ["pending", "accepted", undefined],
    ["downloading", "applying", "download"],
    ["verifying", "applying", "verify"],
    ["installing", "applying", "install"],
    ["restarting", "applying", "start"],
    ["ready-check", "applying", "ready-check"],
    ["completed", "applying", "ready"],
    ["rolled-back", "failed", undefined],
    ["failed", "failed", undefined],
  ] as const)("Should project runtime phase %s", (phase, state, uiPhase) => {
    const operation: UpdateOperation = {
      schema_version: 1,
      operation_id: "runtime-operation",
      revision: 1,
      targets: ["runtime"],
      active_target: "runtime",
      percent: 50,
      runtime: {
        from_version: "1.0.0",
        to_version: "1.1.0",
        release_tag: "v1.1.0",
        asset: "runtime.tar.gz",
        digest: "sha256:runtime",
        install_method: "desktop-app",
        phase: phase as RuntimeOperationPhase,
        daemon_restarted: false,
      },
      holder: null,
      waiting: "",
    };
    expect(projectAppUpdateState(operation)).toMatchObject({
      app_state: "idle",
      runtime_state: state,
      ...(uiPhase ? { phase: uiPhase } : {}),
    });
  });

  it("Should project idle and staged app states", () => {
    expect(projectAppUpdateState(null)).toEqual({ app_state: "idle", runtime_state: "idle" });
    const operation: UpdateOperation = {
      schema_version: 1,
      operation_id: "staged-operation",
      revision: 1,
      targets: ["app"],
      percent: -1,
      app: {
        from_version: "1.0.0",
        to_version: "1.1.0",
        release_tag: "v1.1.0",
        asset: "app.zip",
        digest: "sha256:app",
        attempt_id: "attempt-1",
        phase: "staged",
        consecutive_failures: 0,
      },
      holder: null,
      waiting: "waiting-for-app",
    };
    expect(projectAppUpdateState(operation)).toMatchObject({ app_state: "staged", percent: -1 });
  });

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
      expect(record).toEqual({
        schema_version: 2,
        pid: process.pid,
        started_at: "2026-08-16T00:00:00.000Z",
        app_version: "0.3.0",
        channel: "development",
        diagnostic_report: {
          schema_version: 1,
          boot_id: expect.any(String),
          boot_phase: "product",
          app_version: "0.3.0",
          runtime_version: "0.3.0",
          runtime_owned: true,
          current_error: null,
          previous_crash: null,
        },
        state: "product",
        origin: "http://127.0.0.1:2123",
        owned: true,
        update: {
          app_state: "applying",
          runtime_state: "idle",
          app_available: "0.3.1",
          operation_id: "operation-1",
          phase: "download",
          percent: 42,
        },
      });
    } finally {
      await rm(home, { recursive: true, force: true });
    }
  });

  it("Should project live runtime operation truth", async () => {
    const home = await mkdtemp(join(tmpdir(), "compozy-runtime-state-"));
    const path = join(home, "app.json");
    try {
      const publisher = new AppStatePublisher({
        path,
        appVersion: "0.3.0",
        channel: "development",
        startedAt: new Date("2026-08-16T00:00:00Z"),
      });
      await publisher.setOperation({
        schema_version: 1,
        operation_id: "operation-runtime",
        revision: 3,
        targets: ["runtime"],
        active_target: "runtime",
        percent: 35,
        runtime: {
          from_version: "0.3.0",
          to_version: "0.3.1",
          release_tag: "v0.3.1",
          asset: "compozy-darwin-arm64.tar.gz",
          digest: "sha256:def",
          install_method: "desktop-app",
          phase: "downloading",
          daemon_restarted: false,
        },
        holder: null,
        waiting: "",
      });
      await publisher.publish({ state: "updating", target: "runtime" });

      const record = JSON.parse(await readFile(path, "utf8")) as Record<string, unknown>;
      expect(record.update).toEqual({
        app_state: "idle",
        runtime_state: "applying",
        runtime_available: "0.3.1",
        operation_id: "operation-runtime",
        phase: "download",
        percent: 35,
      });
    } finally {
      await rm(home, { recursive: true, force: true });
    }
  });

  it("Should recover an unclean prior launch and clear the marker on clean shutdown", async () => {
    const home = await mkdtemp(join(tmpdir(), "compozy-startup-marker-"));
    const path = join(home, "app.json");
    const marker = join(home, "desktop-startup.json");
    try {
      const crashed = await AppStatePublisher.create({
        path,
        startupMarker: marker,
        appVersion: "0.3.0",
        channel: "development",
        startedAt: new Date("2026-08-16T00:00:00Z"),
        bootId: "boot-crashed",
      });
      await crashed.publish({ state: "starting", stage: "start", attempt: 1 });

      const recovered = await AppStatePublisher.create({
        path,
        startupMarker: marker,
        appVersion: "0.3.0",
        channel: "development",
        startedAt: new Date("2026-08-16T00:01:00Z"),
        bootId: "boot-current",
      });
      expect(recovered.diagnosticReport().previous_crash).toEqual({
        boot_id: "boot-crashed",
        boot_phase: "starting",
        started_at: "2026-08-16T00:00:00.000Z",
      });
      await recovered.markCleanShutdown();
      await expect(access(marker)).rejects.toMatchObject({ code: "ENOENT" });
    } finally {
      await rm(home, { recursive: true, force: true });
    }
  });
});
