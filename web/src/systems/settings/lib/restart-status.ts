import type { SettingsRestartStatusName } from "../types";

export const RESTART_TERMINAL_STATUSES = [
  "ready",
  "failed",
] as const satisfies readonly SettingsRestartStatusName[];

const TERMINAL_STATUS_SET = new Set<SettingsRestartStatusName>(RESTART_TERMINAL_STATUSES);

export function isTerminalRestartStatus(
  status: SettingsRestartStatusName | string | undefined | null
): boolean {
  if (!status) {
    return false;
  }

  return TERMINAL_STATUS_SET.has(status as SettingsRestartStatusName);
}

export function isSuccessfulRestart(
  status: SettingsRestartStatusName | string | undefined | null
): boolean {
  return status === "ready";
}

export function isFailedRestart(
  status: SettingsRestartStatusName | string | undefined | null
): boolean {
  return status === "failed";
}
