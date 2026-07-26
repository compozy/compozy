import type { SettingsUpdateStatus } from "../types";

export type SettingsUpdateView =
  | { kind: "checking" }
  | { kind: "error"; message: string }
  | { kind: "unavailable"; message: string }
  | { kind: "snapshot"; snapshot: SettingsUpdateStatus; refreshError: string | null };

function updateErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Update check failed";
}

export function settingsUpdateView(input: {
  isLoading: boolean;
  isError: boolean;
  error: unknown;
  data?: SettingsUpdateStatus;
}): SettingsUpdateView {
  if (input.data) {
    return {
      kind: "snapshot",
      snapshot: input.data,
      refreshError: input.isError ? updateErrorMessage(input.error) : null,
    };
  }
  if (input.isLoading) return { kind: "checking" };
  if (input.isError) {
    return { kind: "error", message: updateErrorMessage(input.error) };
  }
  return { kind: "unavailable", message: "Update status is unavailable." };
}
