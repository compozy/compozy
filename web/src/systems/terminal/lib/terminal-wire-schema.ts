/**
 * Control-frame payloads, validated at the socket boundary.
 *
 * A frame that does not match its schema is a protocol error, not a shrug: the
 * screen state and the resync gate are driven by these payloads, so silently
 * accepting a half-understood frame would let the client show a state the
 * daemon never reported.
 */

import { z } from "zod";

import {
  terminalErrorEnvelopeSchema,
  terminalExitCauseSchema,
  terminalModeSchema,
  terminalSignalSchema,
} from "./terminal-contract-schema";
import {
  TERMINAL_MAX_COLS,
  TERMINAL_MAX_ROWS,
  TERMINAL_MIN_COLS,
  TERMINAL_MIN_ROWS,
} from "./terminal-wire";

const U64_MAX = 0xffff_ffff_ffff_ffffn;

/**
 * One u64, two JSON encodings the wire actually uses.
 *
 * Control frames encode the sequence as a decimal string. Some harnesses and
 * HTTP-shaped payloads send the same integer as a JSON number. Both are one
 * field — not an alias — and both become `bigint` before anything reads them.
 */
const sequenceSchema = z
  .union([
    z.string().regex(/^(0|[1-9]\d*)$/),
    z
      .number()
      .int()
      .nonnegative()
      .refine(value => Number.isSafeInteger(value), "sequence is not a safe integer"),
  ])
  .transform(value => BigInt(value))
  .refine(value => value <= U64_MAX, "sequence exceeds u64");
const colsSchema = z.number().int().min(TERMINAL_MIN_COLS).max(TERMINAL_MAX_COLS);
const rowsSchema = z.number().int().min(TERMINAL_MIN_ROWS).max(TERMINAL_MAX_ROWS);

export const terminalAttachedFrameSchema = z.strictObject({
  seq: sequenceSchema,
  truncated: z.boolean(),
  cols: colsSchema,
  rows: rowsSchema,
  mode: terminalModeSchema,
  preamble: z.string().optional(),
});

export const terminalExitFrameSchema = z.strictObject({
  cause: terminalExitCauseSchema,
  exit_code: z.number().int().nonnegative().nullable(),
  signal: terminalSignalSchema.nullable(),
  seq: sequenceSchema,
});

export const terminalErrorFrameSchema = terminalErrorEnvelopeSchema;

export const terminalTitleFrameSchema = z.strictObject({
  title: z.string(),
});

export const terminalResizedFrameSchema = z.strictObject({
  cols: colsSchema,
  rows: rowsSchema,
});

export const terminalGapFrameSchema = z.strictObject({
  from_seq: sequenceSchema,
  to_seq: sequenceSchema,
  dropped_bytes: z.number().int().nonnegative(),
});

export const terminalPresenceFrameSchema = z.strictObject({
  viewers: z.number().int().nonnegative(),
});

export const terminalRedactedInputFrameSchema = z.strictObject({
  seq: sequenceSchema,
  characters: z.number().int().nonnegative(),
});

export type TerminalAttachedFrame = z.infer<typeof terminalAttachedFrameSchema>;
export type TerminalExitFrame = z.infer<typeof terminalExitFrameSchema>;
export type TerminalErrorFrame = z.infer<typeof terminalErrorFrameSchema>;
export type TerminalTitleFrame = z.infer<typeof terminalTitleFrameSchema>;
export type TerminalResizedFrame = z.infer<typeof terminalResizedFrameSchema>;
export type TerminalGapFrame = z.infer<typeof terminalGapFrameSchema>;
export type TerminalPresenceFrame = z.infer<typeof terminalPresenceFrameSchema>;
export type TerminalRedactedInputFrame = z.infer<typeof terminalRedactedInputFrameSchema>;
