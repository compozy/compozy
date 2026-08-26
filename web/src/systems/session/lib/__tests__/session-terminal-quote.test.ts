import { beforeEach, describe, expect, it } from "vitest";

import { sessionStore } from "../../stores/session-store";
import {
  clearSessionTerminalQuote,
  stageSessionTerminalQuote,
  stripTerminalQuote,
} from "../session-terminal-quote";

/**
 * Canonical suite for the terminal → composer handoff (part of UT-117).
 *
 * Invariant: staging a selection for a session produces the canonical block, and
 * removing it strips exactly that block from the draft, leaving whatever the
 * person annotated.
 */

const SESSION_ID = "sess-77ab";

// Both stores are app-wide singletons, so every case starts from a known state
// rather than from whatever the previous one left behind — including a case
// that failed halfway through.
beforeEach(() => {
  clearSessionTerminalQuote(SESSION_ID);
  sessionStore.trigger.composerDraftDiscarded({ sessionId: SESSION_ID });
});

describe("session terminal quote", () => {
  it("Should stage the serialized quote for a session", () => {
    const quote = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 214,
      lines: [
        "12:41:04 [vite] Internal server error: Failed to resolve import",
        '"@compozy/ui/terminal-view" from "src/systems/terminal/terminal-pane.tsx"',
      ],
    });

    expect(quote).toMatchObject({
      terminalId: "term-4f21c9a03b7e",
      fromLine: 214,
      toLine: 215,
    });
    expect(sessionStore.getSnapshot().context.drafts[SESSION_ID]).toBe(quote.text);
  });

  it("Should remove only the block, keeping the annotation", () => {
    const quote = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 12,
      lines: ["FAIL src/api/users.test.ts"],
    });
    const draft = `${quote.text}\n\nWhat is failing here?`;

    expect(stripTerminalQuote(draft, quote)).toBe("What is failing here?");
  });

  it("Should leave a draft alone when its block was already removed", () => {
    const quote = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["one line"],
    });

    expect(stripTerminalQuote("Just my own words", quote)).toBe("Just my own words");
  });

  it("Should keep one quote per session at a time", () => {
    const first = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["first"],
    });
    const second = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-9cd7e14b2a66",
      fromLine: 9,
      lines: ["second"],
    });

    // The gesture is "send this to the conversation", not "collect excerpts".
    expect(second.terminalId).not.toBe(first.terminalId);
    expect(second.text).toContain("term-9cd7e14b2a66");
  });
});

/**
 * The half that actually reaches the agent.
 *
 * A chip above the composer is not the message. Staging has to put the
 * canonical block into the draft the person is about to send, and removing the
 * chip has to take exactly that block back out.
 */
describe("session terminal quote — the composer draft", () => {
  const draft = () => sessionStore.getSnapshot().context.drafts[SESSION_ID] ?? "";

  it("Should put the block in the draft, exactly once", () => {
    const quote = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["one line"],
    });

    expect(draft()).toBe(quote.text);
    expect(draft().match(/<terminal_context /g)).toHaveLength(1);
  });

  it("Should keep what was already typed and add the block after it", () => {
    sessionStore.trigger.composerDraftChanged({
      sessionId: SESSION_ID,
      text: "What is failing here?",
    });

    const quote = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["one line"],
    });

    expect(draft()).toBe(`What is failing here?\n\n${quote.text}`);
  });

  it("Should replace the previous block rather than stack a second one", () => {
    sessionStore.trigger.composerDraftChanged({ sessionId: SESSION_ID, text: "Look at this" });
    stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["first"],
    });
    const second = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-9cd7e14b2a66",
      fromLine: 9,
      lines: ["second"],
    });

    expect(draft()).toBe(`Look at this\n\n${second.text}`);
    expect(draft().match(/<terminal_context /g)).toHaveLength(1);
  });

  it("Should take the block back out when the quote is removed", () => {
    sessionStore.trigger.composerDraftChanged({ sessionId: SESSION_ID, text: "Look at this" });
    stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["one line"],
    });

    clearSessionTerminalQuote(SESSION_ID);

    expect(draft()).toBe("Look at this");
  });
});
