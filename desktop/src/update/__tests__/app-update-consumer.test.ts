import { describe, expect, it } from "vitest";

import { AppUpdateConsumer, type AppUpdateTransitions } from "../app-update-consumer";
import type { AppUpdateInstaller } from "../electron-installer";
import type { AppOperationPhase, UpdateOperation } from "../operation-contract";

function stagedOperation(): UpdateOperation {
  return {
    schema_version: 1,
    operation_id: "operation-1",
    revision: 3,
    targets: ["app"],
    percent: -1,
    app: {
      from_version: "1.0.0",
      to_version: "1.1.0",
      release_tag: "v1.1.0",
      asset: "CompozyOS-1.1.0-arm64.zip",
      digest: "sha256:abc",
      attempt_id: "attempt-1",
      phase: "staged",
      consecutive_failures: 0,
    },
    holder: null,
    waiting: "waiting-for-app",
  };
}

function nextOperation(
  operation: UpdateOperation,
  phase: AppOperationPhase = operation.app?.phase ?? "staged",
  percent = operation.percent,
  watchdogDeadline = operation.app?.watchdog_deadline
): UpdateOperation {
  if (!operation.app) throw new Error("Test operation requires the app track.");
  return {
    ...operation,
    revision: operation.revision + 1,
    active_target: "app",
    percent,
    app: {
      ...operation.app,
      phase,
      ...(watchdogDeadline ? { watchdog_deadline: watchdogDeadline } : {}),
    },
    holder: {
      pid: process.pid,
      pid_start_time: "2026-08-16T00:00:00Z",
      surface: "shell",
      executor_generation: "test",
      lease_expires_at: "2026-08-16T01:00:00Z",
    },
    waiting: "",
  };
}

// Invariant: Update Operation transitions fence every irreversible installer side effect.
// Owner: desktop app-update consumer. Canonical suite: app-update-consumer.test.ts.
describe("app update consumer", () => {
  it("Should journal applying and verify the artifact before installer handoff", async () => {
    const calls: string[] = [];
    let acquireIdentity: Parameters<AppUpdateTransitions["acquire"]>[0] | null = null;
    const phaseTransitions: Parameters<AppUpdateTransitions["phase"]>[0][] = [];
    let verifyRequest: {
      identity: Parameters<AppUpdateTransitions["verifyArtifact"]>[0];
      attemptId: string;
      artifactPath: string;
    } | null = null;
    const transitions: AppUpdateTransitions = {
      async acquire(identity) {
        calls.push("acquire");
        acquireIdentity = identity;
        current = nextOperation(stagedOperation());
        return current;
      },
      async recover() {
        throw new Error("unexpected recover");
      },
      async renew() {
        calls.push("renew");
        return current;
      },
      async phase(transition) {
        calls.push(`phase:${transition.phase}`);
        phaseTransitions.push(transition);
        const base = current;
        current = nextOperation(
          base,
          transition.phase,
          transition.percent,
          transition.watchdogDeadline instanceof Date
            ? transition.watchdogDeadline.toISOString()
            : undefined
        );
        return current;
      },
      async progress(_identity, percent) {
        calls.push(`progress:${percent}`);
        current = nextOperation(current, "applying", percent);
        return current;
      },
      async verifyArtifact(identity, attemptId, artifactPath) {
        calls.push("verify-artifact");
        verifyRequest = { identity, attemptId, artifactPath };
        current = nextOperation(current, "applying", 100);
        return current;
      },
      async fence() {
        calls.push("fence");
        return current;
      },
    };
    let current = stagedOperation();
    const installer: AppUpdateInstaller = {
      async download(_operation, onProgress) {
        calls.push("download");
        await onProgress(50);
        return { artifactPath: "/tmp/CompozyOS-1.1.0-arm64.zip", version: "1.1.0" };
      },
      quitAndInstall() {
        calls.push("quit-and-install");
      },
    };
    const consumer = new AppUpdateConsumer({
      currentVersion: "1.0.0",
      transitions,
      installer,
      now: () => new Date("2026-08-16T00:00:00Z"),
      onError: error => {
        throw error;
      },
    });

    await consumer.handle(current);
    consumer.stop();

    expect(calls).toEqual([
      "acquire",
      "phase:applying",
      "download",
      "progress:50",
      "verify-artifact",
      "phase:installer-handoff",
      "fence",
      "quit-and-install",
    ]);
    expect(acquireIdentity).toEqual({
      operationId: "operation-1",
      revision: 3,
      executorGeneration: expect.any(String),
    });
    expect(phaseTransitions[0]).toMatchObject({
      operationId: "operation-1",
      revision: 4,
      phase: "applying",
      percent: 0,
      watchdogDeadline: new Date("2026-08-16T00:10:00Z"),
    });
    expect(verifyRequest).toMatchObject({
      identity: { operationId: "operation-1", executorGeneration: expect.any(String) },
      attemptId: "attempt-1",
      artifactPath: "/tmp/CompozyOS-1.1.0-arm64.zip",
    });
    expect(phaseTransitions.at(-1)).toMatchObject({
      operationId: "operation-1",
      phase: "installer-handoff",
      percent: 100,
      watchdogDeadline: new Date("2026-08-16T00:05:00Z"),
    });
  });

  it("Should journal a terminal failure when download fails", async () => {
    const phases: AppOperationPhase[] = [];
    let current = stagedOperation();
    const transitions: AppUpdateTransitions = {
      async acquire() {
        current = nextOperation(current);
        return current;
      },
      async recover() {
        throw new Error("unexpected recover");
      },
      async renew() {
        return current;
      },
      async phase(transition) {
        phases.push(transition.phase);
        current = nextOperation(current, transition.phase, transition.percent);
        return current;
      },
      async progress() {
        throw new Error("unexpected progress");
      },
      async verifyArtifact() {
        throw new Error("unexpected verification");
      },
      async fence() {
        throw new Error("unexpected fence");
      },
    };
    const failures: Error[] = [];
    const consumer = new AppUpdateConsumer({
      currentVersion: "1.0.0",
      transitions,
      installer: {
        async download() {
          throw new Error("injected download failure");
        },
        quitAndInstall() {
          throw new Error("unexpected install");
        },
      },
      onError: error => failures.push(error),
    });

    await consumer.handle(current);
    consumer.stop();

    expect(phases).toEqual(["applying", "failed"]);
    expect(failures).toHaveLength(1);
    expect(failures[0]?.message).toBe("injected download failure");
  });

  it("Should recover an expired staged lease before installing", async () => {
    const calls: string[] = [];
    let current = nextOperation(stagedOperation());
    current = {
      ...current,
      holder: { ...current.holder!, lease_expires_at: "2026-08-16T09:59:00Z" },
    };
    const transitions: AppUpdateTransitions = {
      async acquire() {
        throw new Error("unexpected acquire");
      },
      async recover() {
        calls.push("recover");
        current = nextOperation(current);
        return current;
      },
      async renew() {
        return current;
      },
      async phase(transition) {
        calls.push(`phase:${transition.phase}`);
        current = nextOperation(current, transition.phase, transition.percent);
        return current;
      },
      async progress(_identity, percent) {
        current = nextOperation(current, "applying", percent);
        return current;
      },
      async verifyArtifact() {
        calls.push("verify-artifact");
        return current;
      },
      async fence() {
        calls.push("fence");
        return current;
      },
    };
    const consumer = new AppUpdateConsumer({
      currentVersion: "1.0.0",
      transitions,
      installer: {
        async download() {
          calls.push("download");
          return { artifactPath: "/tmp/CompozyOS-1.1.0-arm64.zip", version: "1.1.0" };
        },
        quitAndInstall() {
          calls.push("quit-and-install");
        },
      },
      now: () => new Date("2026-08-16T10:00:00Z"),
      onError: error => {
        throw error;
      },
    });

    await consumer.handle(current);
    consumer.stop();

    expect(calls).toEqual([
      "recover",
      "phase:applying",
      "download",
      "verify-artifact",
      "phase:installer-handoff",
      "fence",
      "quit-and-install",
    ]);
  });
});
