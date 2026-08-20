import { z } from "zod";

export const globalShortcutRegistrationStatusSchema = z.enum([
  "registered",
  "failed_in_use",
  "failed_permission",
  "unsupported",
]);

export type GlobalShortcutRegistrationStatus = z.infer<
  typeof globalShortcutRegistrationStatusSchema
>;

export const globalShortcutRegistrationSchema = z.strictObject({
  command_id: z.string().trim().min(1),
  intended_chord: z.string().trim().min(1),
  active_chord: z.string().trim().min(1).optional(),
  status: globalShortcutRegistrationStatusSchema,
  reason: z.string().optional(),
  settings_url: z.string().optional(),
});
