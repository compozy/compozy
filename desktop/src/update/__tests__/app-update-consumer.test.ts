import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterEach, describe, expect, it, vi } from "vitest";

import { AppUpdateConsumer, type AppUpdateTransitions } from "../app-update-consumer";
import type { AppUpdateInstaller } from "../electron-installer";
import type { AppOperationPhase, UpdateOperation } from "../operation-contract";
import { OperationWatcher } from "../operation-watcher";

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
  afterEach(() => vi.useRealTimers());
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
    let finishInstall!: () => void;
    const installFinished = new Promise<void>(resolve => {
      finishInstall = resolve;
    });
    const installer: AppUpdateInstaller = {
      async download(_operation, onProgress) {
        calls.push("download");
        await onProgress(50);
        return { artifactPath: "/tmp/CompozyOS-1.1.0-arm64.zip", version: "1.1.0" };
      },
      async quitAndInstall() {
        calls.push("quit-and-install:start");
        await installFinished;
        calls.push("quit-and-install:complete");
      },
      async prepareInstall() {},
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

    const handling = consumer.handle(current);
    await expect.poll(() => calls.at(-1)).toBe("quit-and-install:start");
    expect(calls).not.toContain("quit-and-install:complete");
    finishInstall();
    await handling;
    consumer.stop();

    expect(calls).toEqual([
      "acquire",
      "phase:applying",
      "download",
      "progress:50",
      "verify-artifact",
      "phase:installer-handoff",
      "fence",
      "quit-and-install:start",
      "quit-and-install:complete",
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

  it.each([true, false])(
    "Should acknowledge a download failure only after journaling it (%s)",
    async journalSucceeds => {
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
          if (transition.phase === "failed" && !journalSucceeds)
            throw new Error("journal unavailable");
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
          async quitAndInstall() {
            throw new Error("unexpected install");
          },
          async prepareInstall() {},
        },
        onError: error => failures.push(error),
      });

      const accepted = await consumer.handle(current);
      consumer.stop();

      expect(phases).toEqual(["applying", "failed"]);
      expect(accepted).toBe(journalSucceeds);
      expect(failures).toHaveLength(1);
      expect(failures[0]?.message).toBe(
        journalSucceeds
          ? "injected download failure"
          : "The app update failed and its outcome could not be journaled."
      );
    }
  );

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
        async quitAndInstall() {
          calls.push("quit-and-install");
        },
        async prepareInstall() {},
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

  // Invariant: an installer deadline settles the operation and a late I/O completion cannot install.
  // Owner: desktop update consumer. Canonical suite: app-update-consumer.test.ts.
  it.each(["download", "progressing-download", "download-limit", "handoff"] as const)(
    "Should fail a stalled %s and fence its late completion",
    async step => {
      vi.useFakeTimers();
      let current = stagedOperation();
      let release!: () => void;
      const blocked = new Promise<void>(resolve => {
        release = resolve;
      });
      const phases: AppOperationPhase[] = [];
      const errors: Error[] = [];
      let signal: AbortSignal | undefined;
      let progress: ((percent: number) => Promise<void>) | undefined;
      let lastError: string | undefined;
      let installs = 0;
      const transitions: AppUpdateTransitions = {
        async acquire() {
          current = nextOperation(current);
          return current;
        },
        async recover() {
          throw new Error("unexpected recovery");
        },
        async renew() {
          current = nextOperation(current);
          return current;
        },
        async phase(transition) {
          phases.push(transition.phase);
          lastError = transition.lastError;
          current = nextOperation(current, transition.phase, transition.percent);
          return current;
        },
        async progress() {
          return current;
        },
        async verifyArtifact() {
          return current;
        },
        async fence() {
          return current;
        },
      };
      const consumer = new AppUpdateConsumer({
        currentVersion: "1.0.0",
        transitions,
        installer: {
          async download(_operation, onProgress, cancellation) {
            if (step !== "handoff") {
              progress = onProgress;
              signal = cancellation;
              await blocked;
            }
            return { artifactPath: "/tmp/CompozyOS-1.1.0-arm64.zip", version: "1.1.0" };
          },
          async prepareInstall(cancellation) {
            if (step === "handoff") {
              signal = cancellation;
              await blocked;
            }
          },
          async quitAndInstall() {
            installs++;
          },
        },
        onError: error => errors.push(error),
      });
      const handling = consumer.handle(current);
      await vi.advanceTimersByTimeAsync(0);
      if (step === "progressing-download") {
        for (let percent = 1; percent <= 3; percent++) {
          await vi.advanceTimersByTimeAsync(45_000);
          await progress!(percent);
          expect(current.app?.phase).toBe("applying");
          expect(signal?.aborted).toBe(false);
        }
        // A repeated percentage is not a download heartbeat.
        await vi.advanceTimersByTimeAsync(45_000);
        await progress!(3);
        await vi.advanceTimersByTimeAsync(15_001);
      } else if (step === "download-limit") {
        // Real progress cannot extend the absolute ten-minute ceiling.
        for (let percent = 1; percent < 20; percent++) {
          await vi.advanceTimersByTimeAsync(30_000);
          await progress!(percent);
          expect(current.app?.phase).toBe("applying");
        }
        await vi.advanceTimersByTimeAsync(30_001);
      } else {
        await vi.advanceTimersByTimeAsync((step === "download" ? 1 : 5) * 60 * 1000 + 1);
      }
      expect(current.app?.phase).toBe("failed");
      expect(signal?.aborted).toBe(true);
      expect(errors).toHaveLength(1);
      expect(lastError).toBe(
        step === "handoff"
          ? "The app installer did not become ready. Retry the update."
          : "The app update download timed out. Retry the update."
      );
      release();
      await handling;
      await vi.advanceTimersByTimeAsync(0);
      expect(installs).toBe(0);
      expect(phases.at(-1)).toBe("failed");
      consumer.stop();
    }
  );

  // Invariant: an unchanged staged journal is retried after a transient acquire failure.
  // Owner: desktop consumer/watch loop. Canonical suite: app-update-consumer.test.ts.
  it("Should retry acquisition on the next poll without reinstalling a journaled failure", async () => {
    const directory = await mkdtemp(join(tmpdir(), "compozy-update-watch-"));
    const path = join(directory, "operation.json");
    let current = stagedOperation();
    let acquisitions = 0;
    let downloads = 0;
    const errors: Error[] = [];
    const persist = async () => {
      await writeFile(path, JSON.stringify(current));
      return current;
    };
    await persist();
    const consumer = new AppUpdateConsumer({
      currentVersion: "1.0.0",
      transitions: {
        async acquire() {
          acquisitions++;
          if (acquisitions === 1) throw new Error("transient acquire failure");
          current = nextOperation(current);
          return await persist();
        },
        async recover() {
          throw new Error("unexpected recover");
        },
        async renew() {
          return current;
        },
        async phase(transition) {
          current = nextOperation(current, transition.phase, transition.percent);
          return await persist();
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
      },
      installer: {
        async download() {
          downloads++;
          throw new Error("download unavailable");
        },
        async prepareInstall() {
          throw new Error("unexpected preparation");
        },
        async quitAndInstall() {
          throw new Error("unexpected installation");
        },
      },
      onError: error => errors.push(error),
    });
    const watcher = new OperationWatcher({
      path,
      pollIntervalMs: 20,
      onOperation: async operation => await consumer.handle(operation),
      onError: error => errors.push(error),
    });
    try {
      watcher.start();
      await expect.poll(() => current.app?.phase).toBe("failed");
      await new Promise(resolve => setTimeout(resolve, 80));
      expect(acquisitions).toBe(2);
      expect(downloads).toBe(1);
      expect(errors.map(error => error.message)).toEqual([
        "transient acquire failure",
        "download unavailable",
      ]);
    } finally {
      watcher.stop();
      consumer.stop();
      await rm(directory, { recursive: true, force: true });
    }
  });
});
