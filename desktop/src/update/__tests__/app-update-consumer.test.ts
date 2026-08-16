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
describe("app update consumer", () => {
  it("Should journal applying and verify the artifact before installer handoff", async () => {
    const calls: string[] = [];
    const transitions: AppUpdateTransitions = {
      async acquire() {
        calls.push("acquire");
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
      async verifyArtifact() {
        calls.push("verify-artifact");
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
  });
});
