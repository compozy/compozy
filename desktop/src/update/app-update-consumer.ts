import { randomUUID } from "node:crypto";

import { decideAppUpdateAction } from "./consumer-policy";
import type { AppUpdateInstaller } from "./electron-installer";
import type { UpdateOperation } from "./operation-contract";
import type { TransitionIdentity, UpdateTransitionPort } from "./transition-client";

const APPLY_WATCHDOG_MS = 10 * 60 * 1000;
const DOWNLOAD_IDLE_MS = 60 * 1000;
const INSTALLER_WATCHDOG_MS = 5 * 60 * 1000;
const QUIT_WATCHDOG_MS = 30 * 1000;
const LEASE_RENEWAL_MS = 30 * 1000;

export type AppUpdateTransitions = UpdateTransitionPort;

function identity(operation: UpdateOperation, executorGeneration: string): TransitionIdentity {
  return {
    operationId: operation.operation_id,
    revision: operation.revision,
    executorGeneration,
  };
}

export class AppUpdateConsumer {
  readonly #currentVersion: string;
  readonly #transitions: UpdateTransitionPort;
  readonly #installer: AppUpdateInstaller;
  readonly #now: () => Date;
  readonly #onError: (error: Error) => void;
  readonly #executorGeneration = randomUUID();
  #activeOperation = "";
  #deadlineTimer: NodeJS.Timeout | null = null;

  constructor(options: {
    currentVersion: string;
    transitions: UpdateTransitionPort;
    installer: AppUpdateInstaller;
    now?: () => Date;
    onError: (error: Error) => void;
  }) {
    this.#currentVersion = options.currentVersion;
    this.#transitions = options.transitions;
    this.#installer = options.installer;
    this.#now = options.now ?? (() => new Date());
    this.#onError = options.onError;
  }

  async handle(operation: UpdateOperation | null): Promise<boolean> {
    this.#clearDeadline();
    if (!operation) return true;
    if (this.#activeOperation === operation.operation_id) return false;
    const decision = decideAppUpdateAction(operation, this.#currentVersion, this.#now());
    if (decision.kind === "wait") {
      this.#scheduleDeadline(operation);
      return true;
    }
    if (decision.kind === "ignore") return true;
    this.#activeOperation = operation.operation_id;
    try {
      switch (decision.kind) {
        case "install":
          await this.#install(operation, false);
          break;
        case "recover-install":
          await this.#install(operation, true);
          break;
        case "verify-restart":
          await this.#verifyRestart(operation);
          break;
        case "fail-silent-install":
          await this.#failInterrupted(operation, decision.reason);
          break;
      }
      return true;
    } catch (error) {
      this.#onError(error instanceof Error ? error : new Error(String(error)));
      return false;
    } finally {
      this.#activeOperation = "";
    }
  }

  stop(): void {
    this.#clearDeadline();
  }

  async #install(operation: UpdateOperation, recover: boolean): Promise<void> {
    const appOperation = operation.app;
    if (!appOperation) return;
    let current = recover
      ? await this.#transitions.recover(identity(operation, this.#executorGeneration))
      : await this.#transitions.acquire(identity(operation, this.#executorGeneration));
    current = await this.#transitions.phase({
      ...identity(current, this.#executorGeneration),
      phase: "applying",
      percent: 0,
      watchdogDeadline: new Date(this.#now().getTime() + APPLY_WATCHDOG_MS),
    });
    let transitionQueue = Promise.resolve();
    const updateCurrent = (run: (value: UpdateOperation) => Promise<UpdateOperation>) => {
      const result = transitionQueue.then(async () => {
        current = await run(current);
        return current;
      });
      transitionQueue = result.then(
        () => undefined,
        () => undefined
      );
      return result;
    };
    const renewal = setInterval(() => {
      void updateCurrent(
        async value => await this.#transitions.renew(identity(value, this.#executorGeneration))
      ).catch(error => this.#onError(error instanceof Error ? error : new Error(String(error))));
    }, LEASE_RENEWAL_MS);
    renewal.unref();
    try {
      let lastProgress = 0;
      const downloaded = await withInstallerDeadline(
        APPLY_WATCHDOG_MS,
        "The app update download timed out. Retry the update.",
        async (signal, heartbeat) =>
          await this.#installer.download(
            appOperation,
            async percent => {
              signal.throwIfAborted();
              if (percent > lastProgress) {
                lastProgress = percent;
                heartbeat();
              }
              await updateCurrent(async value => {
                signal.throwIfAborted();
                return await this.#transitions.progress(
                  identity(value, this.#executorGeneration),
                  percent
                );
              });
            },
            signal
          ),
        DOWNLOAD_IDLE_MS
      );
      if (
        downloaded.version.trim().replace(/^v/, "") !==
        appOperation.to_version.trim().replace(/^v/, "")
      ) {
        throw new Error("The downloaded app version does not match the update operation.");
      }
      await transitionQueue;
      current = await updateCurrent(
        async value =>
          await this.#transitions.verifyArtifact(
            identity(value, this.#executorGeneration),
            appOperation.attempt_id,
            downloaded.artifactPath
          )
      );
      current = await updateCurrent(
        async value =>
          await this.#transitions.phase({
            ...identity(value, this.#executorGeneration),
            phase: "installer-handoff",
            percent: 100,
            watchdogDeadline: new Date(this.#now().getTime() + INSTALLER_WATCHDOG_MS),
          })
      );
      await withInstallerDeadline(
        INSTALLER_WATCHDOG_MS,
        "The app installer did not become ready. Retry the update.",
        async signal => await this.#installer.prepareInstall(signal)
      );
      clearInterval(renewal);
      await transitionQueue;
      await this.#transitions.fence(identity(current, this.#executorGeneration));
      await withInstallerDeadline(
        QUIT_WATCHDOG_MS,
        "The app could not restart to finish its update. Retry the update.",
        async signal => await this.#installer.quitAndInstall(signal)
      );
      this.#scheduleDeadline(current);
    } catch (error) {
      clearInterval(renewal);
      await transitionQueue;
      await this.#recordInstallFailure(current, error);
      this.#onError(error instanceof Error ? error : new Error(String(error)));
    } finally {
      clearInterval(renewal);
    }
  }

  async #recordInstallFailure(operation: UpdateOperation, cause: unknown): Promise<void> {
    try {
      await this.#transitions.phase({
        ...identity(operation, this.#executorGeneration),
        phase: "failed",
        percent: -1,
        lastError:
          cause instanceof InstallerDeadlineError
            ? cause.message
            : "The app update could not be installed.",
        outcome: "failed",
        watchdogDeadline: "clear",
        incrementFailures: true,
      });
    } catch (transitionError) {
      throw new AggregateError(
        [cause, transitionError],
        "The app update failed and its outcome could not be journaled."
      );
    }
  }

  async #verifyRestart(operation: UpdateOperation): Promise<void> {
    let current = await this.#transitions.recover(identity(operation, this.#executorGeneration));
    current = await this.#transitions.phase({
      ...identity(current, this.#executorGeneration),
      phase: "restarted",
      percent: 100,
      watchdogDeadline: "clear",
    });
    await this.#transitions.phase({
      ...identity(current, this.#executorGeneration),
      phase: "verified",
      percent: 100,
      outcome: "updated",
      resetFailures: true,
    });
  }

  async #failInterrupted(operation: UpdateOperation, reason: string): Promise<void> {
    const current = await this.#transitions.recover(identity(operation, this.#executorGeneration));
    await this.#transitions.phase({
      ...identity(current, this.#executorGeneration),
      phase: "failed",
      percent: -1,
      lastError: reason,
      outcome: "failed",
      watchdogDeadline: "clear",
      incrementFailures: true,
    });
  }

  #scheduleDeadline(operation: UpdateOperation): void {
    const deadline =
      operation.app?.watchdog_deadline ??
      (operation.app?.phase === "staged" ? operation.holder?.lease_expires_at : undefined);
    if (!deadline) return;
    const delay = Math.max(0, Date.parse(deadline) - this.#now().getTime() + 25);
    this.#deadlineTimer = setTimeout(() => void this.handle(operation), delay);
    this.#deadlineTimer.unref();
  }

  #clearDeadline(): void {
    if (this.#deadlineTimer) clearTimeout(this.#deadlineTimer);
    this.#deadlineTimer = null;
  }
}

class InstallerDeadlineError extends Error {}

async function withInstallerDeadline<T>(
  duration: number,
  message: string,
  run: (signal: AbortSignal, heartbeat: () => void) => Promise<T>,
  inactivityDuration?: number
): Promise<T> {
  const controller = new AbortController();
  const expired = new Promise<never>((_resolve, reject) => {
    controller.signal.addEventListener("abort", () => reject(controller.signal.reason), {
      once: true,
    });
  });
  const expire = () => controller.abort(new InstallerDeadlineError(message));
  const timer = setTimeout(expire, duration);
  const inactivityTimer = inactivityDuration ? setTimeout(expire, inactivityDuration) : null;
  timer.unref();
  inactivityTimer?.unref();
  try {
    return await Promise.race([
      run(controller.signal, () => {
        if (!controller.signal.aborted) inactivityTimer?.refresh();
      }),
      expired,
    ]);
  } finally {
    clearTimeout(timer);
    if (inactivityTimer) clearTimeout(inactivityTimer);
  }
}
