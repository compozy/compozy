import { randomUUID } from "node:crypto";

import { writeFileAtomic } from "../files/atomic-write";
import type { UpdateOperation } from "../update/operation-contract";
import {
  readStartupMarker,
  removeStartupMarker,
  writeStartupMarker,
  type StartupMarker,
} from "./startup-marker";

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
  readonly pct?: number;
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
  readonly previous_crash: StartupMarker | null;
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
  const phase =
    operation.active_target === "runtime" ? operation.runtime?.phase : operation.app?.phase;
  if (!phase) return undefined;
  if (phase === "downloading") return "download";
  if (phase === "verifying") return "verify";
  if (phase === "installing") return "install";
  if (phase === "restarting") return "start";
  if (phase === "ready-check") return "ready-check";
  if (phase === "completed") return "ready";
  if (phase === "applying") return operation.percent >= 100 ? "verify" : "download";
  if (phase === "installer-handoff") return "install";
  if (phase === "restarted") return "start";
  if (phase === "verified") return "ready";
  return undefined;
}

function appTrackState(operation: UpdateOperation): string {
  const app = operation.app;
  if (!app) return "idle";
  if (operation.waiting === "waiting-for-app" || app.phase === "staged") return "staged";
  switch (app.phase) {
    case "failed":
      return "failed";
    case "pending":
      return "accepted";
    default:
      return "applying";
  }
}

function runtimeTrackState(operation: UpdateOperation): string {
  const runtime = operation.runtime;
  if (!runtime) return "idle";
  switch (runtime.phase) {
    case "failed":
    case "rolled-back":
      return "failed";
    case "pending":
      return "accepted";
    default:
      return "applying";
  }
}

export function projectAppUpdateState(operation: UpdateOperation | null): AppUpdateState {
  if (!operation) return { app_state: "idle", runtime_state: "idle" };
  const app = operation.app;
  const runtime = operation.runtime;
  const phase = operationPhase(operation);
  return {
    app_state: appTrackState(operation),
    runtime_state: runtimeTrackState(operation),
    ...(app ? { app_available: app.to_version } : {}),
    ...(runtime ? { runtime_available: runtime.to_version } : {}),
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
  readonly #bootId: string;
  readonly #startupMarker: string | null;
  readonly #previousCrash: StartupMarker | null;
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
    bootId?: string;
    startupMarker?: string;
    previousCrash?: StartupMarker | null;
    onPublished?: (snapshot: ShellSnapshot) => void;
  }) {
    this.#path = options.path;
    this.#appVersion = options.appVersion;
    this.#channel = options.channel;
    this.#startedAt = options.startedAt.toISOString();
    this.#bootId = options.bootId ?? randomUUID();
    this.#startupMarker = options.startupMarker ?? null;
    this.#previousCrash = options.previousCrash ?? null;
    this.#onPublished = options.onPublished ?? (() => undefined);
  }

  static async create(options: {
    path: string;
    appVersion: string;
    channel: string;
    startedAt: Date;
    bootId?: string;
    startupMarker: string;
    onPublished?: (snapshot: ShellSnapshot) => void;
  }): Promise<AppStatePublisher> {
    const previousCrash = await readStartupMarker(options.startupMarker);
    const publisher = new AppStatePublisher({ ...options, previousCrash });
    await publisher.#persistMarker();
    return publisher;
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
      previous_crash: this.#previousCrash,
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

  async markCleanShutdown(): Promise<void> {
    if (this.#startupMarker) await removeStartupMarker(this.#startupMarker);
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
      update: projectAppUpdateState(this.#operation),
    };
    const data = `${JSON.stringify(record, null, 2)}\n`;
    const write = this.#publication.then(async () => {
      await this.#persistMarker();
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

  async #persistMarker(): Promise<void> {
    if (!this.#startupMarker) return;
    await writeStartupMarker(this.#startupMarker, {
      boot_id: this.#bootId,
      boot_phase: this.#snapshot.state,
      started_at: this.#startedAt,
    });
  }
}
