import { randomUUID } from "node:crypto";

import { writeFileAtomic } from "../files/atomic-write";
import type { UpdateOperation } from "../update/operation-contract";

export type ShellStateName =
  | "resolving"
  | "provisioning"
  | "starting"
  | "attaching"
  | "product"
  | "updating"
  | "disconnected"
  | "skew"
  | "error";

export interface PublicShellError {
  readonly code: string;
  readonly safe_message: string;
  readonly log_path: string;
}

export interface ShellSnapshot {
  readonly state: ShellStateName;
  readonly stage?: "download" | "verify" | "install" | "start" | "ready";
  readonly attempt?: number;
  readonly origin?: string;
  readonly owned?: boolean;
  readonly target?: "app" | "runtime";
  readonly runtime?: string;
  readonly needed?: string;
  readonly newer?: boolean;
  readonly error?: PublicShellError;
}

export interface DiagnosticReport {
  readonly schema_version: 1;
  readonly boot_id: string;
  readonly boot_phase: string;
  readonly app_version: string;
  readonly runtime_version: string | null;
  readonly runtime_owned: boolean | null;
  readonly current_error: { readonly code: string; readonly safe_message: string } | null;
  readonly previous_crash: null;
}

interface AppUpdateState {
  readonly app_state: string;
  readonly runtime_state: string;
  readonly app_available?: string;
  readonly runtime_available?: string;
  readonly operation_id?: string;
  readonly phase?: "download" | "verify" | "install" | "start" | "ready-check" | "ready";
  readonly percent?: number;
}

function operationPhase(operation: UpdateOperation): AppUpdateState["phase"] {
  const phase = operation.app?.phase;
  if (!phase) return undefined;
  if (phase === "applying") return operation.percent >= 100 ? "verify" : "download";
  if (phase === "installer-handoff") return "install";
  if (phase === "restarted") return "start";
  if (phase === "verified") return "ready";
  return undefined;
}

function updateState(operation: UpdateOperation | null): AppUpdateState {
  if (!operation) return { app_state: "idle", runtime_state: "idle" };
  const app = operation.app;
  const appState = !app
    ? "idle"
    : operation.waiting === "waiting-for-app" || app.phase === "staged"
      ? "staged"
      : app.phase === "failed"
        ? "failed"
        : app.phase === "pending"
          ? "accepted"
          : "applying";
  const phase = operationPhase(operation);
  return {
    app_state: appState,
    runtime_state: "idle",
    ...(app ? { app_available: app.to_version } : {}),
    operation_id: operation.operation_id,
    ...(phase ? { phase } : {}),
    percent: operation.percent,
  };
}

export class AppStatePublisher {
  readonly #path: string;
  readonly #appVersion: string;
  readonly #channel: string;
  readonly #startedAt: string;
  readonly #bootId = randomUUID();
  readonly #onPublished: (snapshot: ShellSnapshot) => void;
  #snapshot: ShellSnapshot = { state: "resolving" };
  #operation: UpdateOperation | null = null;
  #runtimeVersion: string | null = null;
  #runtimeOwned: boolean | null = null;
  #publication = Promise.resolve();

  constructor(options: {
    path: string;
    appVersion: string;
    channel: string;
    startedAt: Date;
    onPublished?: (snapshot: ShellSnapshot) => void;
  }) {
    this.#path = options.path;
    this.#appVersion = options.appVersion;
    this.#channel = options.channel;
    this.#startedAt = options.startedAt.toISOString();
    this.#onPublished = options.onPublished ?? (() => undefined);
  }

  snapshot(): ShellSnapshot {
    return this.#snapshot;
  }

  diagnosticReport(): DiagnosticReport {
    const error = this.#snapshot.error;
    return {
      schema_version: 1,
      boot_id: this.#bootId,
      boot_phase: this.#snapshot.state,
      app_version: this.#appVersion,
      runtime_version: this.#runtimeVersion,
      runtime_owned: this.#runtimeOwned,
      current_error: error ? { code: error.code, safe_message: error.safe_message } : null,
      previous_crash: null,
    };
  }

  async publish(snapshot: ShellSnapshot): Promise<void> {
    this.#snapshot = snapshot;
    await this.#persist();
  }

  async setRuntime(version: string | null, owned: boolean): Promise<void> {
    this.#runtimeVersion = version;
    this.#runtimeOwned = owned;
    await this.#persist();
  }

  async setOperation(operation: UpdateOperation | null): Promise<void> {
    this.#operation = operation;
    await this.#persist();
  }

  async #persist(): Promise<void> {
    const snapshot = this.#snapshot;
    const record = {
      schema_version: 2,
      pid: process.pid,
      started_at: this.#startedAt,
      app_version: this.#appVersion,
      channel: this.#channel,
      diagnostic_report: this.diagnosticReport(),
      ...snapshot,
      update: updateState(this.#operation),
    };
    const data = `${JSON.stringify(record, null, 2)}\n`;
    const write = this.#publication.then(async () => {
      await writeFileAtomic(this.#path, data, 0o600);
      this.#onPublished(snapshot);
    });
    // Keep the serialization chain usable; this caller still receives the write failure below.
    this.#publication = write.then(
      () => undefined,
      () => undefined
    );
    await write;
  }
}
