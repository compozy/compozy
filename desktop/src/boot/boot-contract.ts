export const BOOT_METHOD_VALUES = [
  "retry",
  "diagnose",
  "copy_diagnostics",
  "export_diagnostics",
  "open_logs",
  "quit",
] as const;
export type BootMethod = (typeof BOOT_METHOD_VALUES)[number];
export const BOOT_METHODS = new Set<string>(BOOT_METHOD_VALUES);
export type ForwardedBootMethod = Exclude<BootMethod, "open_logs" | "quit">;
const FORWARDED_BOOT_METHODS = new Set<string>([
  "retry",
  "diagnose",
  "copy_diagnostics",
  "export_diagnostics",
] satisfies readonly BootMethod[]);

export function isForwardedBootMethod(value: string): value is ForwardedBootMethod {
  return FORWARDED_BOOT_METHODS.has(value);
}
const STATES = new Set([
  "resolving",
  "provisioning",
  "starting",
  "attaching",
  "product",
  "updating",
  "disconnected",
  "skew",
  "error",
]);
const STAGES = new Set(["download", "verify", "install", "start", "ready"]);
const TARGETS = new Set(["app", "runtime"]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function validError(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.code === "string" &&
    value.code.trim() !== "" &&
    typeof value.safe_message === "string" &&
    typeof value.log_path === "string"
  );
}

function validPreviousCrash(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.boot_id === "string" &&
    value.boot_id.trim() !== "" &&
    typeof value.boot_phase === "string" &&
    STATES.has(value.boot_phase) &&
    typeof value.started_at === "string" &&
    Number.isFinite(Date.parse(value.started_at))
  );
}

function validDiagnosticError(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.code === "string" &&
    value.code.trim() !== "" &&
    typeof value.safe_message === "string"
  );
}

export function validState(value: unknown): boolean {
  if (!isRecord(value) || typeof value.state !== "string" || !STATES.has(value.state)) return false;
  return (
    (value.stage === undefined || (typeof value.stage === "string" && STAGES.has(value.stage))) &&
    (value.attempt === undefined ||
      (typeof value.attempt === "number" &&
        Number.isSafeInteger(value.attempt) &&
        value.attempt >= 1 &&
        value.attempt <= 5)) &&
    (value.origin === undefined ||
      (typeof value.origin === "string" &&
        /^https?:\/\/(?:127\.0\.0\.1|localhost|\[::1\])(?::\d+)?(?:\/|$)/u.test(value.origin))) &&
    (value.owned === undefined || typeof value.owned === "boolean") &&
    (value.target === undefined ||
      (typeof value.target === "string" && TARGETS.has(value.target))) &&
    (value.pct === undefined ||
      (typeof value.pct === "number" &&
        Number.isFinite(value.pct) &&
        value.pct >= 0 &&
        value.pct <= 100)) &&
    (value.runtime === undefined || typeof value.runtime === "string") &&
    (value.needed === undefined || typeof value.needed === "string") &&
    (value.newer === undefined || typeof value.newer === "boolean") &&
    (value.error === undefined || validError(value.error))
  );
}

function validDiagnosticReport(value: unknown): boolean {
  if (!isRecord(value) || value.schema_version !== 1) return false;
  return (
    typeof value.boot_id === "string" &&
    value.boot_id.trim() !== "" &&
    typeof value.boot_phase === "string" &&
    STATES.has(value.boot_phase) &&
    typeof value.app_version === "string" &&
    value.app_version.trim() !== "" &&
    (value.runtime_version === null || typeof value.runtime_version === "string") &&
    (value.runtime_owned === null || typeof value.runtime_owned === "boolean") &&
    (value.current_error === null || validDiagnosticError(value.current_error)) &&
    (value.previous_crash === null || validPreviousCrash(value.previous_crash))
  );
}

export function validParams(method: BootMethod, value: unknown): boolean {
  if (!isRecord(value)) return false;
  if (method === "export_diagnostics") {
    return value.consent === true && Object.keys(value).length === 1;
  }
  return Object.keys(value).length === 0;
}

export function validResponse(method: BootMethod, value: unknown): boolean {
  if (!isRecord(value)) return false;
  switch (method) {
    case "retry":
      return value.ok === true || value.accepted === true;
    case "diagnose":
      return validDiagnosticReport(value);
    case "copy_diagnostics":
      return value.copied === true;
    case "export_diagnostics":
      return (
        typeof value.bundle_path === "string" &&
        value.bundle_path.trim() !== "" &&
        typeof value.bytes === "number" &&
        Number.isSafeInteger(value.bytes) &&
        value.bytes >= 0
      );
    case "open_logs":
      return value.opened === true;
    case "quit":
      return value.quitting === true;
  }
}
