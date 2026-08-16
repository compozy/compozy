import { randomUUID } from "node:crypto";

import { decideAppUpdateAction } from "./consumer-policy";
import type { AppUpdateInstaller } from "./electron-installer";
import type { UpdateOperation } from "./operation-contract";
import type { PhaseTransition, UpdateTransitionClient } from "./transition-client";

const APPLY_WATCHDOG_MS = 10 * 60 * 1000;
const INSTALLER_WATCHDOG_MS = 5 * 60 * 1000;
const LEASE_RENEWAL_MS = 30 * 1000;

interface OperationIdentity {
  readonly operationId: string;
  readonly revision: number;
  readonly executorGeneration: string;
}

export interface AppUpdateTransitions {
  acquire(identity: OperationIdentity): Promise<UpdateOperation>;
  recover(identity: OperationIdentity): Promise<UpdateOperation>;
  renew(identity: OperationIdentity): Promise<UpdateOperation>;
  fence(identity: OperationIdentity): Promise<UpdateOperation>;
  progress(identity: OperationIdentity, percent: number): Promise<UpdateOperation>;
  phase(transition: PhaseTransition): Promise<UpdateOperation>;
  verifyArtifact(
    identity: OperationIdentity,
    attemptId: string,
    artifactPath: string
  ): Promise<UpdateOperation>;
}

function identity(operation: UpdateOperation, executorGeneration: string): OperationIdentity {
  return {
    operationId: operation.operation_id,
    revision: operation.revision,
    executorGeneration,
  };
}

export class AppUpdateConsumer {
  readonly #currentVersion: string;
  readonly #transitions: AppUpdateTransitions;
  readonly #installer: AppUpdateInstaller;
  readonly #now: () => Date;
  readonly #onError: (error: Error) => void;
  readonly #executorGeneration = randomUUID();
  #activeOperation = "";
  #deadlineTimer: NodeJS.Timeout | null = null;

  constructor(options: {
    currentVersion: string;
    transitions: UpdateTransitionClient | AppUpdateTransitions;
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

  async handle(operation: UpdateOperation | null): Promise<void> {
    this.#clearDeadline();
    if (!operation || this.#activeOperation === operation.operation_id) return;
    const decision = decideAppUpdateAction(operation, this.#currentVersion, this.#now());
    if (decision.kind === "wait") {
      this.#scheduleDeadline(operation);
      return;
    }
    if (decision.kind === "ignore") return;
    this.#activeOperation = operation.operation_id;
    try {
      switch (decision.kind) {
        case "install":
          await this.#install(operation);
          break;
        case "verify-restart":
          await this.#verifyRestart(operation);
          break;
        case "fail-silent-install":
          await this.#failInterrupted(operation, decision.reason);
          break;
      }
    } catch (error) {
      this.#onError(error instanceof Error ? error : new Error(String(error)));
    } finally {
      this.#activeOperation = "";
    }
  }

  stop(): void {
    this.#clearDeadline();
  }

  async #install(operation: UpdateOperation): Promise<void> {
    const appOperation = operation.app;
    if (!appOperation) return;
    let current = await this.#transitions.acquire(identity(operation, this.#executorGeneration));
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
      const downloaded = await this.#installer.download(appOperation, async percent => {
        await updateCurrent(
          async value =>
            await this.#transitions.progress(identity(value, this.#executorGeneration), percent)
        );
      });
      if (
        downloaded.version.trim().replace(/^v/, "") !==
        appOperation.to_version.trim().replace(/^v/, "")
      ) {
        throw new Error("The downloaded app version does not match the update operation.");
      }
      clearInterval(renewal);
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
      await this.#transitions.fence(identity(current, this.#executorGeneration));
      this.#installer.quitAndInstall();
      this.#scheduleDeadline(current);
    } catch (error) {
      await this.#recordInstallFailure(current, error);
      throw error;
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
        lastError: "The app update could not be installed.",
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
    const deadline = operation.app?.watchdog_deadline;
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
