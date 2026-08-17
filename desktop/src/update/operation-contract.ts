export const UPDATE_OPERATION_SCHEMA_VERSION = 1;

const APP_OPERATION_PHASE_VALUES = [
  "pending",
  "staged",
  "applying",
  "installer-handoff",
  "restarted",
  "verified",
  "failed",
] as const;
export type AppOperationPhase = (typeof APP_OPERATION_PHASE_VALUES)[number];

const RUNTIME_OPERATION_PHASE_VALUES = [
  "pending",
  "downloading",
  "verifying",
  "installing",
  "restarting",
  "ready-check",
  "completed",
  "rolled-back",
  "failed",
] as const;
export type RuntimeOperationPhase = (typeof RUNTIME_OPERATION_PHASE_VALUES)[number];

const UPDATE_TARGET_VALUES = ["runtime", "app"] as const;
export type UpdateTarget = (typeof UPDATE_TARGET_VALUES)[number];
const WAITING_STATE_VALUES = ["", "waiting-for-app"] as const;
export type WaitingState = (typeof WAITING_STATE_VALUES)[number];

export interface UpdateHolder {
  readonly pid: number;
  readonly pid_start_time: string;
  readonly surface: "cli" | "daemon" | "web" | "shell";
  readonly executor_generation: string;
  readonly lease_expires_at: string;
}

export interface AppOperationState {
  readonly from_version: string;
  readonly to_version: string;
  readonly release_tag: string;
  readonly asset: string;
  readonly digest: string;
  readonly attempt_id: string;
  readonly phase: AppOperationPhase;
  readonly consecutive_failures: number;
  readonly watchdog_deadline?: string;
}

export interface RuntimeOperationState {
  readonly from_version: string;
  readonly to_version: string;
  readonly release_tag: string;
  readonly asset: string;
  readonly digest: string;
  readonly install_method: string;
  readonly backup_path?: string;
  readonly phase: RuntimeOperationPhase;
  readonly daemon_restarted: boolean;
}

export interface UpdateOperation {
  readonly schema_version: typeof UPDATE_OPERATION_SCHEMA_VERSION;
  readonly operation_id: string;
  readonly revision: number;
  readonly targets: readonly UpdateTarget[];
  readonly active_target?: UpdateTarget;
  readonly percent: number;
  readonly runtime?: RuntimeOperationState;
  readonly app?: AppOperationState;
  readonly holder: UpdateHolder | null;
  readonly waiting: WaitingState;
}

const APP_PHASES = new Set<string>(APP_OPERATION_PHASE_VALUES);
const RUNTIME_PHASES = new Set<string>(RUNTIME_OPERATION_PHASE_VALUES);
const UPDATE_TARGETS = new Set<unknown>(UPDATE_TARGET_VALUES);
const WAITING_STATES = new Set<unknown>(WAITING_STATE_VALUES);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function parseUpdateOperation(value: unknown): UpdateOperation {
  if (!isRecord(value)) throw new Error("The update operation is invalid.");
  if (value.schema_version !== UPDATE_OPERATION_SCHEMA_VERSION) {
    throw new Error(
      `The update operation schema version ${String(value.schema_version)} is not supported.`
    );
  }
  if (
    typeof value.operation_id !== "string" ||
    value.operation_id.trim() === "" ||
    typeof value.revision !== "number" ||
    !Number.isSafeInteger(value.revision) ||
    value.revision < 1 ||
    typeof value.percent !== "number" ||
    value.percent < -1 ||
    value.percent > 100 ||
    !Array.isArray(value.targets) ||
    !value.targets.every(target => UPDATE_TARGETS.has(target)) ||
    !WAITING_STATES.has(value.waiting)
  ) {
    throw new Error("The update operation is invalid.");
  }
  if (value.runtime !== undefined) parseRuntimeOperation(value.runtime);
  if (value.app !== undefined) validateAppOperation(value.app);
  return value as unknown as UpdateOperation;
}

function parseRuntimeOperation(value: unknown): RuntimeOperationState {
  if (!isRecord(value)) throw new Error("The runtime update operation is invalid.");
  for (const field of [
    "from_version",
    "to_version",
    "release_tag",
    "asset",
    "digest",
    "install_method",
  ] as const) {
    if (typeof value[field] !== "string" || value[field].trim() === "") {
      throw new Error("The runtime update operation is invalid.");
    }
  }
  if (
    typeof value.phase !== "string" ||
    !RUNTIME_PHASES.has(value.phase as RuntimeOperationPhase) ||
    typeof value.daemon_restarted !== "boolean" ||
    (value.backup_path !== undefined && typeof value.backup_path !== "string")
  ) {
    throw new Error("The runtime update operation is invalid.");
  }
  return value as unknown as RuntimeOperationState;
}

function validateAppOperation(value: unknown): void {
  if (!isRecord(value)) throw new Error("The app update operation is invalid.");
  for (const field of [
    "from_version",
    "to_version",
    "release_tag",
    "asset",
    "digest",
    "attempt_id",
  ] as const) {
    if (typeof value[field] !== "string" || value[field].trim() === "") {
      throw new Error("The app update operation is invalid.");
    }
  }
  if (
    typeof value.phase !== "string" ||
    !APP_PHASES.has(value.phase as AppOperationPhase) ||
    typeof value.consecutive_failures !== "number" ||
    !Number.isSafeInteger(value.consecutive_failures) ||
    value.consecutive_failures < 0 ||
    (value.watchdog_deadline !== undefined && typeof value.watchdog_deadline !== "string")
  ) {
    throw new Error("The app update operation is invalid.");
  }
}
