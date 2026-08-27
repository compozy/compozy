// Invariant: malformed JSON and non-objects fail at the draft boundary; objects
// pass through for daemon semantics. Owning layer: `parseExpectDraft`.
// Canonical suite: this file — no prior expect-draft suite existed.
import { describe, expect, it } from "vitest";

import { parseExpectDraft } from "../expect-draft";

describe("parseExpectDraft", () => {
  it("Should treat an empty draft as an omitted contract", () => {
    expect(parseExpectDraft("")).toEqual({ ok: true });
    expect(parseExpectDraft("   ")).toEqual({ ok: true });
  });

  it("Should reject malformed JSON at the caret", () => {
    expect(parseExpectDraft("{")).toEqual({ ok: false, message: "That is not valid JSON." });
  });

  it("Should reject a JSON value that is not an object", () => {
    expect(parseExpectDraft("[]")).toEqual({
      ok: false,
      message: "A result contract must be a JSON object.",
    });
    expect(parseExpectDraft('"ok"')).toEqual({
      ok: false,
      message: "A result contract must be a JSON object.",
    });
  });

  it("Should pass a JSON object through for the daemon to judge", () => {
    expect(parseExpectDraft('{"verdict":"ok"}')).toEqual({
      ok: true,
      value: { verdict: "ok" },
    });
  });
});
