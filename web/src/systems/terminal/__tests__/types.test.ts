import { describe, expect, expectTypeOf, it } from "vitest";

import {
  terminalErrorCodeSchema,
  type GeneratedTerminalErrorCode,
  type TerminalErrorCode,
} from "../lib/terminal-contract-schema";

/**
 * Invariant: the terminal domain error vocabulary is closed against the
 * generated OpenAPI reason-code union and rejects codes outside that set.
 * Owner: `terminal-contract-schema` (web client contract).
 * Canonical suite: this file.
 */
describe("terminal contract types", () => {
  it("Should keep the domain error schema exhaustive of the generated union", () => {
    expectTypeOf<TerminalErrorCode>().toEqualTypeOf<GeneratedTerminalErrorCode>();
    expectTypeOf<GeneratedTerminalErrorCode>().toEqualTypeOf<TerminalErrorCode>();
  });

  it("Should accept a generated domain code and reject a transport code", () => {
    expect(terminalErrorCodeSchema.safeParse("terminal_not_found")).toEqual({
      success: true,
      data: "terminal_not_found",
    });
    expect(terminalErrorCodeSchema.safeParse("service_unavailable").success).toBe(false);
  });
});
