export const UPDATE_OPERATION_SCHEMA_VERSION = 1;

export type AppOperationPhase =
  | "pending"
  | "staged"
  | "applying"
  | "installer-handoff"
  | "restarted"
  | "verified"
  | "failed";

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

export interface UpdateOperation {
  readonly schema_version: 1;
  readonly operation_id: string;
  readonly revision: number;
  readonly targets: readonly ("runtime" | "app")[];
  readonly active_target?: "runtime" | "app";
  readonly percent: number;
  readonly app?: AppOperationState;
  readonly holder: UpdateHolder | null;
  readonly waiting: "" | "waiting-for-app";
}

const APP_PHASES = new Set<AppOperationPhase>([
  "pending",
  "staged",
  "applying",
  "installer-handoff",
  "restarted",
  "verified",
  "failed",
]);

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
    !value.targets.every(target => target === "runtime" || target === "app") ||
    (value.waiting !== "" && value.waiting !== "waiting-for-app")
  ) {
    throw new Error("The update operation is invalid.");
  }
  if (value.app !== undefined) parseAppOperation(value.app);
  return value as unknown as UpdateOperation;
}

function parseAppOperation(value: unknown): AppOperationState {
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
  return value as unknown as AppOperationState;
}
