export type SettingsSaveBarState =
  | { kind: "idle" }
  | { kind: "dirty"; warnings: string[] }
  | { kind: "invalid"; warnings: string[] }
  | { kind: "saving" }
  | { kind: "error"; message: string }
  | { kind: "saved"; message: string }
  | { kind: "warning"; message: string; warnings: string[] };

export interface SettingsSaveStateInput {
  isDirty: boolean;
  isInvalid?: boolean;
  isSaving: boolean;
  error?: string | null;
  warnings?: string[];
  lastAppliedLabel?: string | null;
  showSaved: boolean;
}

export function deriveSettingsSaveBarState({
  isDirty,
  isInvalid = false,
  isSaving,
  error,
  warnings = [],
  lastAppliedLabel,
  showSaved,
}: SettingsSaveStateInput): SettingsSaveBarState {
  if (isSaving) return { kind: "saving" };
  if (error) return { kind: "error", message: error };
  if (isInvalid) return { kind: "invalid", warnings };
  if (isDirty) return { kind: "dirty", warnings };
  if (warnings.length > 0) {
    return {
      kind: "warning",
      message: lastAppliedLabel ?? "Saved with warnings",
      warnings,
    };
  }
  if (showSaved) return { kind: "saved", message: lastAppliedLabel ?? "Saved" };
  return { kind: "idle" };
}
