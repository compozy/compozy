/**
 * Control-frame payloads, validated at the socket boundary.
 *
 * A frame that does not match its schema is a protocol error, not a shrug: the
 * lease UI and the resync gate are both driven by these payloads, so silently
 * accepting a half-understood frame would let the client show a state the
 * daemon never claimed.
 */

import { z } from "zod";

import {
  terminalActorKindSchema,
  terminalLeaseStateSchema,
  terminalModeSchema,
} from "./terminal-wire-enums";
import {
  TERMINAL_MAX_COLS,
  TERMINAL_MAX_ROWS,
  TERMINAL_MIN_COLS,
  TERMINAL_MIN_ROWS,
} from "./terminal-wire";

const sequenceSchema = z.number().int().nonnegative();
const colsSchema = z.number().int().min(TERMINAL_MIN_COLS).max(TERMINAL_MAX_COLS);
const rowsSchema = z.number().int().min(TERMINAL_MIN_ROWS).max(TERMINAL_MAX_ROWS);

export const terminalAttachedFrameSchema = z.object({
  seq: sequenceSchema,
  truncated: z.boolean(),
  cols: colsSchema,
  rows: rowsSchema,
  lease: terminalLeaseStateSchema,
  mode: terminalModeSchema,
});

export const terminalExitFrameSchema = z.object({
  cause: z.enum(["exited", "signaled", "unknown"]),
  exit_code: z.number().int().nonnegative().nullable(),
  signal: z.string().nullable(),
  seq: sequenceSchema,
});

export const terminalErrorFrameSchema = z.object({
  code: z.string(),
  message: z.string(),
});

export const terminalTitleFrameSchema = z.object({
  title: z.string(),
});

export const terminalResizedFrameSchema = z.object({
  cols: colsSchema,
  rows: rowsSchema,
});

export const terminalGapFrameSchema = z.object({
  from_seq: sequenceSchema,
  to_seq: sequenceSchema,
  dropped_bytes: z.number().int().positive(),
});

export const terminalPresenceFrameSchema = z.object({
  viewers: z.number().int().nonnegative(),
});

/** The only authority on who holds the write lease. */
export const terminalOwnerFrameSchema = z.discriminatedUnion("lease", [
  z.object({
    lease: z.literal("available"),
    actor_kind: z.never().optional(),
    actor_id: z.never().optional(),
    reason: z.string().optional(),
  }),
  z.object({
    lease: z.literal("human_owned"),
    actor_kind: z.literal(terminalActorKindSchema.enum.human),
    actor_id: z.string().min(1),
    reason: z.string().optional(),
  }),
  z.object({
    lease: z.literal("agent_owned"),
    actor_kind: z.literal(terminalActorKindSchema.enum.agent),
    actor_id: z.string().min(1),
    reason: z.string().optional(),
  }),
]);

export type TerminalAttachedFrame = z.infer<typeof terminalAttachedFrameSchema>;
export type TerminalExitFrame = z.infer<typeof terminalExitFrameSchema>;
export type TerminalErrorFrame = z.infer<typeof terminalErrorFrameSchema>;
export type TerminalTitleFrame = z.infer<typeof terminalTitleFrameSchema>;
export type TerminalResizedFrame = z.infer<typeof terminalResizedFrameSchema>;
export type TerminalGapFrame = z.infer<typeof terminalGapFrameSchema>;
export type TerminalPresenceFrame = z.infer<typeof terminalPresenceFrameSchema>;
export type TerminalOwnerFrame = z.infer<typeof terminalOwnerFrameSchema>;
