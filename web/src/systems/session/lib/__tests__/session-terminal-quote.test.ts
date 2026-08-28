import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  holdPendingTerminalQuote,
  peekPendingTerminalQuote,
  takePendingTerminalQuote,
} from "@/systems/terminal/parts";

import { sessionStore } from "../../stores/session-store";
import { createTerminalQuoteHostActions } from "../session-terminal-quote-actions";
import {
  clearSessionTerminalQuote,
  peekSessionTerminalQuote,
  restorePendingTerminalQuoteAfterFailedCreate,
  stageChosenSessionTerminalQuote,
  stagePendingTerminalQuoteForSession,
  stageSessionTerminalQuote,
} from "../session-terminal-quote";
import {
  applyTerminalQuoteToPromptMessage,
  composeQuotedPrompt,
  splitQuotedPrompt,
} from "../session-terminal-quote-prompt";

/**
 * Canonical suite for the terminal → composer handoff (part of UT-117).
 *
 * Invariant: staging a selection for a session produces the canonical block
 * without writing that envelope into the composer draft. Send concatenates
 * envelope + annotation.
 */

const SESSION_ID = "sess-77ab";

beforeEach(() => {
  clearSessionTerminalQuote(SESSION_ID);
  takePendingTerminalQuote();
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
    expect(peekSessionTerminalQuote(SESSION_ID)?.text).toBe(quote.text);
    expect(sessionStore.getSnapshot().context.drafts[SESSION_ID]).toBeUndefined();
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

    expect(second.terminalId).not.toBe(first.terminalId);
    expect(peekSessionTerminalQuote(SESSION_ID)?.text).toContain("term-9cd7e14b2a66");
  });
});

/**
 * The half that actually reaches the agent.
 *
 * A chip above the composer is not the message. Send concatenates the
 * canonical block with whatever the person annotated.
 */
describe("session terminal quote — the composer draft", () => {
  const draft = () => sessionStore.getSnapshot().context.drafts[SESSION_ID] ?? "";

  it("Should leave the composer draft as the annotation only", () => {
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

    expect(draft()).toBe("What is failing here?");
    expect(draft()).not.toContain("<terminal_context");
    expect(composeQuotedPrompt(draft(), quote))
      .toBe(`<terminal_context terminal="term-4f21c9a03b7e" lines="1-1">
1 | one line
</terminal_context>

What is failing here?`);
  });

  it("Should send the envelope alone when the annotation is empty", () => {
    const quote = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["one line"],
    });

    expect(composeQuotedPrompt("", quote)).toBe(quote.text);
  });

  it("Should apply the staged quote when the outgoing message is prepared", () => {
    const quote = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["one line"],
    });
    const prepared = applyTerminalQuoteToPromptMessage(SESSION_ID, {
      id: "message-quote",
      parts: [{ text: "What failed?", type: "text" }],
      role: "user",
    });

    expect(prepared.parts).toEqual([{ text: `${quote.text}\n\nWhat failed?`, type: "text" }]);
  });

  it("Should keep the annotation when the quote is removed", () => {
    sessionStore.trigger.composerDraftChanged({ sessionId: SESSION_ID, text: "Look at this" });
    stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["one line"],
    });

    clearSessionTerminalQuote(SESSION_ID);

    expect(draft()).toBe("Look at this");
    expect(peekSessionTerminalQuote(SESSION_ID)).toBeNull();
  });
});

describe("session terminal quote — pending create / chosen session", () => {
  const OTHER_ID = "sess-other";

  beforeEach(() => {
    clearSessionTerminalQuote(OTHER_ID);
  });

  it("Should stage a held quote only onto the session create just produced", () => {
    const quote = stageSessionTerminalQuote({
      sessionId: OTHER_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 12,
      lines: ["FAIL src/api/users.test.ts"],
    });
    clearSessionTerminalQuote(OTHER_ID);
    holdPendingTerminalQuote(quote);

    expect(peekPendingTerminalQuote()?.terminalId).toBe("term-4f21c9a03b7e");
    expect(peekSessionTerminalQuote(SESSION_ID)).toBeNull();
    expect(peekSessionTerminalQuote(OTHER_ID)).toBeNull();

    expect(stagePendingTerminalQuoteForSession(SESSION_ID)?.text).toBe(quote.text);
    expect(peekPendingTerminalQuote()).toBeNull();
    expect(peekSessionTerminalQuote(SESSION_ID)?.text).toBe(quote.text);
    expect(peekSessionTerminalQuote(OTHER_ID)).toBeNull();
  });

  it("Should not give a pending quote to another live session", () => {
    const quote = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["one line"],
    });
    clearSessionTerminalQuote(SESSION_ID);
    holdPendingTerminalQuote(quote);

    expect(peekSessionTerminalQuote(SESSION_ID)).toBeNull();
    expect(peekSessionTerminalQuote(OTHER_ID)).toBeNull();
    expect(peekPendingTerminalQuote()?.text).toBe(quote.text);

    stagePendingTerminalQuoteForSession(OTHER_ID);

    expect(peekSessionTerminalQuote(SESSION_ID)).toBeNull();
    expect(peekSessionTerminalQuote(OTHER_ID)?.text).toBe(quote.text);
    expect(peekPendingTerminalQuote()).toBeNull();
  });

  it("Should leave pending in place when a session is only observed", () => {
    const quote = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["one line"],
    });
    clearSessionTerminalQuote(SESSION_ID);
    holdPendingTerminalQuote(quote);

    expect(peekSessionTerminalQuote(SESSION_ID)).toBeNull();
    expect(peekSessionTerminalQuote(OTHER_ID)).toBeNull();
    expect(peekPendingTerminalQuote()?.text).toBe(quote.text);
  });

  it("Should stage a chosen-session quote without using the create pending slot", () => {
    const quote = stageSessionTerminalQuote({
      sessionId: OTHER_ID,
      terminalId: "term-9cd7e14b2a66",
      fromLine: 4,
      lines: ["chosen"],
    });
    clearSessionTerminalQuote(OTHER_ID);
    holdPendingTerminalQuote(quote);

    stageChosenSessionTerminalQuote(SESSION_ID, quote);

    expect(peekSessionTerminalQuote(SESSION_ID)?.text).toBe(quote.text);
    expect(peekPendingTerminalQuote()?.text).toBe(quote.text);
  });

  it("Should put a failed-create quote back when the pending slot is empty", () => {
    const quote = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 12,
      lines: ["FAIL src/api/users.test.ts"],
    });
    clearSessionTerminalQuote(SESSION_ID);

    restorePendingTerminalQuoteAfterFailedCreate(quote);

    expect(peekPendingTerminalQuote()?.text).toBe(quote.text);
  });

  it("Should leave a newer pending quote when a failed create tries to restore", () => {
    const failed = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 12,
      lines: ["first"],
    });
    const newer = stageSessionTerminalQuote({
      sessionId: OTHER_ID,
      terminalId: "term-9cd7e14b2a66",
      fromLine: 40,
      lines: ["second"],
    });
    clearSessionTerminalQuote(SESSION_ID);
    clearSessionTerminalQuote(OTHER_ID);
    holdPendingTerminalQuote(newer);

    restorePendingTerminalQuoteAfterFailedCreate(failed);

    expect(peekPendingTerminalQuote()?.text).toBe(newer.text);
  });

  it("Should stage the captured create quote without taking a later pending slot", () => {
    const captured = stageSessionTerminalQuote({
      sessionId: OTHER_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 12,
      lines: ["first"],
    });
    const later = stageSessionTerminalQuote({
      sessionId: OTHER_ID,
      terminalId: "term-9cd7e14b2a66",
      fromLine: 40,
      lines: ["later"],
    });
    clearSessionTerminalQuote(OTHER_ID);
    holdPendingTerminalQuote(later);

    stageChosenSessionTerminalQuote(SESSION_ID, captured);

    expect(peekSessionTerminalQuote(SESSION_ID)?.text).toBe(captured.text);
    expect(peekPendingTerminalQuote()?.text).toBe(later.text);
  });

  it("Should hand the quote to the picker instead of holding it for the next thread", () => {
    const openSessionPicker = vi.fn();
    const actions = createTerminalQuoteHostActions({
      activateSession: vi.fn(),
      getActiveSessionId: () => null,
      openSessionPicker,
      startSessionWithQuote: vi.fn(),
    });

    actions.onChooseSession("term-4f21c9a03b7e", { startLine: 1, text: "one line" });

    expect(openSessionPicker).toHaveBeenCalledOnce();
    const handed = openSessionPicker.mock.calls[0]?.[0];
    expect(handed?.terminalId).toBe("term-4f21c9a03b7e");
    expect(peekPendingTerminalQuote()).toBeNull();
  });
});

describe("session terminal quote — recovery and edit split", () => {
  it("Should keep the annotation and quote identity separate", () => {
    const quote = stageSessionTerminalQuote({
      sessionId: SESSION_ID,
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["one line"],
    });
    const wire = composeQuotedPrompt("What failed?", quote);
    const split = splitQuotedPrompt(wire);

    expect(split.quote?.text).toBe(quote.text);
    expect(split.annotation).toBe("What failed?");
    expect(split.annotation).not.toContain("<terminal_context");
  });
});
