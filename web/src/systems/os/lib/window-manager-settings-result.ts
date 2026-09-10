import { z } from "zod";

const applySchema = z.object({
  applied: z.boolean(),
  lifecycle: z.enum([
    "live",
    "live-add",
    "live-remove-if-unused",
    "restart-required",
    "session-rebind",
  ]),
  apply_record_id: z.string(),
  active_generation: z.number().int(),
  active_config_hash: z.string(),
  next_action: z.enum(["none", "restart-daemon", "new-session", "retry"]),
  restart_required: z.boolean().optional(),
  warnings: z.array(z.string()).optional(),
  partial_failures: z
    .array(
      z.object({
        subsystem: z.string(),
        diagnostic: z.object({ title: z.string(), message: z.string() }),
      })
    )
    .optional(),
  skipped: z.boolean().optional(),
  skipped_reason: z.string().optional(),
});

export type WindowManagerSettingsApply = z.infer<typeof applySchema>;

/** Validate the daemon receipt before any caller interprets HTTP success as live application. */
export function parseWindowManagerSettingsApply(value: unknown): WindowManagerSettingsApply {
  return applySchema.parse(value);
}

/** Describe persisted state separately from application failure or a pending operator action. */
export function windowManagerApplyMessage(result: WindowManagerSettingsApply): string {
  if (result.partial_failures?.length || result.next_action === "retry") {
    const details = [
      ...(result.partial_failures?.map(failure => failure.diagnostic.message) ?? []),
      ...(result.warnings ?? []),
    ].join(" · ");
    const action =
      result.next_action === "restart-daemon" || result.restart_required
        ? "Restart CompozyOS to apply."
        : result.next_action === "new-session"
          ? "Start a new session to apply."
          : "Resolve the problem and retry.";
    return `Settings saved, but apply failed. ${details ? `${details} ` : ""}${action}`;
  }
  if (result.restart_required || result.next_action === "restart-daemon") {
    return "Settings saved. Restart CompozyOS to apply.";
  }
  if (result.next_action === "new-session") return "Settings saved. New sessions use this config.";
  if (result.skipped) return "No config changes detected.";
  if (!result.applied) return "Settings saved, but not applied. Retry to apply.";
  return "Settings saved and applied.";
}

/** Identify failed or unverified live application while allowing explicit deferred actions. */
export function windowManagerApplyFailed(result: WindowManagerSettingsApply): boolean {
  return (
    Boolean(result.partial_failures?.length) ||
    result.next_action === "retry" ||
    (!result.applied &&
      !result.skipped &&
      !result.restart_required &&
      result.next_action === "none")
  );
}
