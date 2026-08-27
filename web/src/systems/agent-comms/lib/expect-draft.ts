import type { CreateSingleCallRequest } from "../types";

type CallExpect = NonNullable<CreateSingleCallRequest["expect"]>;

/**
 * Pre-flight for a hand-written result contract.
 *
 * Deliberately shallow: it catches only malformed JSON, which is the one failure
 * the operator can see and fix at the caret. Anything that parses goes to the
 * daemon, which owns whether the shape is a usable contract — both accepted
 * forms (an example object or a full JSON Schema) normalize to the same thing
 * server-side, and second-guessing that here would mean maintaining a copy of
 * the validator that could disagree with it.
 */
export type ExpectDraftResult = { ok: true; value?: CallExpect } | { ok: false; message: string };

export function parseExpectDraft(raw: string): ExpectDraftResult {
  const trimmed = raw.trim();
  // An omitted contract is valid: a call without one still runs, it just gets an
  // unchecked answer.
  if (trimmed === "") return { ok: true };
  try {
    const value: unknown = JSON.parse(trimmed);
    if (!isRecord(value)) {
      return { ok: false, message: "A result contract must be a JSON object." };
    }
    return { ok: true, value };
  } catch (error) {
    return {
      ok: false,
      message: error instanceof Error ? error.message : "That is not valid JSON.",
    };
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
