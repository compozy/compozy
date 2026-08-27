import { z } from "zod";

import type { CreateSingleCallRequest } from "../types";

type CallExpect = NonNullable<CreateSingleCallRequest["expect"]>;

/**
 * Pre-flight for a hand-written result contract.
 *
 * Deliberately shallow: it catches only malformed JSON, which is the one failure
 * the operator can see and fix at the caret. Anything that parses as an object
 * goes to the daemon, which owns whether the shape is a usable contract.
 */
export type ExpectDraftResult = { ok: true; value?: CallExpect } | { ok: false; message: string };

const expectDraftSchema = z.record(z.string(), z.unknown());

export function parseExpectDraft(raw: string): ExpectDraftResult {
  const trimmed = raw.trim();
  // An omitted contract is valid: a call without one still runs, it just gets an
  // unchecked answer.
  if (trimmed === "") return { ok: true };
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch (error) {
    if (!(error instanceof SyntaxError)) throw error;
    return { ok: false, message: "That is not valid JSON." };
  }
  const result = expectDraftSchema.safeParse(parsed);
  if (!result.success) {
    return { ok: false, message: "A result contract must be a JSON object." };
  }
  return { ok: true, value: result.data };
}
