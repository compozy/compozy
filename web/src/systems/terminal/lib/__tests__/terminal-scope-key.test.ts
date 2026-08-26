import { describe, expect, it } from "vitest";

import {
  terminalInstanceKey,
  terminalInstanceKeyInScope,
  terminalScopeKey,
} from "../terminal-scope-key";

describe("terminal scope keys", () => {
  it("Should key one terminal buffer to one profile", () => {
    const work = terminalScopeKey("ws-atlas", "work");
    const personal = terminalScopeKey("ws-atlas", "personal");
    const workBuffer = terminalInstanceKey("ws-atlas", "work", "term-1");

    expect(workBuffer).not.toEqual(terminalInstanceKey("ws-atlas", "personal", "term-1"));
    expect(terminalInstanceKeyInScope(workBuffer, work)).toBe(true);
    expect(terminalInstanceKeyInScope(workBuffer, personal)).toBe(false);
  });

  it("Should keep delimiter-like scope values collision-free", () => {
    expect(terminalScopeKey("ws-a", "b-c")).not.toEqual(terminalScopeKey("ws-a-b", "c"));
    expect(terminalScopeKey("ws", "atlas work")).not.toEqual(terminalScopeKey("ws atlas", "work"));
  });
});
