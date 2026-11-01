/**
 * Dialog header tone model, shared by `ConfirmDialog` and `EntityDialogHeader`.
 *
 * `neutral` is the identity well as-is (the mint `KindIcon` well) with a muted
 * eyebrow — the default for every non-attention dialog. `brand` is reserved
 * for a dialog that is itself about something needing attention; `warning`
 * and `destructive` keep their semantic tint on the same plate geometry.
 */
export type DialogTone = "destructive" | "warning" | "brand" | "neutral";

export const DIALOG_TONE_EYEBROW: Record<DialogTone, string> = {
  destructive: "text-destructive",
  warning: "text-warning",
  brand: "text-brand-strong",
  neutral: "text-muted-foreground",
};

/** Extra classes for the `KindIcon tone="well"` plate; `neutral` adds none. */
export const DIALOG_ICON_WELL_TONE: Record<DialogTone, string | undefined> = {
  brand: "bg-brand-tint text-brand-strong",
  neutral: undefined,
  warning: "bg-warning-tint text-warning",
  destructive: "bg-destructive-tint text-destructive",
};
