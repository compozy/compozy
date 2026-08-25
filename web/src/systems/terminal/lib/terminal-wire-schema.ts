/**
 * Control-frame payloads, validated at the socket boundary.
 *
 * A frame that does not match its schema is a protocol error, not a shrug: the
 * lease UI and the resync gate are both driven by these payloads, so silently
 * accepting a half-understood frame would let the client show a state the
 * daemon never claimed.
 */

import { z } from "zod";

const leaseStateSchema = z.enum(["agent_owned", "human_owned", "available"]);
const actorKindSchema = z.enum(["human", "agent", "system"]);

export const terminalAttachedFrameSchema = z.object({
  seq: z.number(),
  truncated: z.boolean(),
  cols: z.number(),
  rows: z.number(),
  lease: leaseStateSchema,
  mode: z.enum(["pty", "pipe"]),
});

export const terminalExitFrameSchema = z.object({
  cause: z.enum(["exited", "signaled", "unknown"]),
  exit_code: z.number().nullable(),
  signal: z.string().nullable(),
  seq: z.number(),
});

export const terminalErrorFrameSchema = z.object({
  code: z.string(),
  message: z.string(),
});

export const terminalTitleFrameSchema = z.object({
  title: z.string(),
});

export const terminalResizedFrameSchema = z.object({
  cols: z.number(),
  rows: z.number(),
});

export const terminalGapFrameSchema = z.object({
  from_seq: z.number(),
  to_seq: z.number(),
  dropped_bytes: z.number(),
});

/** The only authority on who holds the write lease. */
export const terminalOwnerFrameSchema = z.object({
  lease: leaseStateSchema,
  actor_kind: actorKindSchema.optional(),
  actor_id: z.string().optional(),
  reason: z.string().optional(),
});

export type TerminalAttachedFrame = z.infer<typeof terminalAttachedFrameSchema>;
export type TerminalExitFrame = z.infer<typeof terminalExitFrameSchema>;
export type TerminalErrorFrame = z.infer<typeof terminalErrorFrameSchema>;
export type TerminalTitleFrame = z.infer<typeof terminalTitleFrameSchema>;
export type TerminalResizedFrame = z.infer<typeof terminalResizedFrameSchema>;
export type TerminalGapFrame = z.infer<typeof terminalGapFrameSchema>;
export type TerminalOwnerFrame = z.infer<typeof terminalOwnerFrameSchema>;
