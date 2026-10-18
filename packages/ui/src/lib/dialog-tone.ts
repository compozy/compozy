/**
 * Dialog header tone model, shared by `ConfirmDialog` and `EntityDialogHeader`.
 *
 * `neutral` is the identity well as-is (the mint `KindIcon` well) with a muted
 * eyebrow — the default for every non-attention dialog. `accent` is reserved
 * for a dialog that is itself about something needing attention; `warning`
 * and `danger` keep their semantic tint on the same plate geometry.
 */
export type DialogTone = "danger" | "warning" | "accent" | "neutral";

export const DIALOG_TONE_EYEBROW: Record<DialogTone, string> = {
  danger: "text-danger",
  warning: "text-warning",
  accent: "text-accent-strong",
  neutral: "text-muted",
};

/** Extra classes for the `KindIcon tone="well"` plate; `neutral` adds none. */
export const DIALOG_ICON_WELL_TONE: Record<DialogTone, string | undefined> = {
  accent: "bg-accent-tint text-accent-strong",
  neutral: undefined,
  warning: "bg-warning-tint text-warning",
  danger: "bg-danger-tint text-danger",
};
