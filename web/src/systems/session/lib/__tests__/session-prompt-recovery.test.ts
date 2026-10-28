// Suite: session prompt recovery
// Invariant: only a locally rejected prompt restores its latest annotation,
// quote identity, and files once; the editor text never contains the envelope.
// Accepted prompts and unsubscribed listeners cannot restore a draft; a
// rejection with no composer mounted waits for that session's next composer;
// overlapping prompts of one session settle only their own identity; a send
// whose acknowledgment was lost with no page mounted waits for that session's
// next page with its identity intact.
// Owning layer: systems/session/lib prompt recovery lifecycle.
import { describe, expect, it, vi } from "vitest";

import { buildTerminalQuote } from "@/systems/terminal/parts";

import { SessionPromptRecovery } from "../session-prompt-recovery";
import { composeQuotedPrompt } from "../session-terminal-quote-prompt";

const SCOPE = { workspaceId: "workspace-atlas", sessionId: "session-primary" };

const messages = [
  {
    id: "prompt-1",
    parts: [
      { text: "First line", type: "text" as const },
      { text: "Second line", type: "text" as const },
    ],
    role: "user" as const,
  },
];

describe("SessionPromptRecovery", () => {
  it("Should restore the staged user text and files once", () => {
    const recovery = new SessionPromptRecovery();
    const listener = vi.fn();
    const file = new File(["notes"], "notes.txt", { type: "text/plain" });
    recovery.subscribe(SCOPE, listener);

    recovery.stage(SCOPE, "prompt-1", { messages });
    recovery.recover(SCOPE, "prompt-1", [file]);
    recovery.recover(SCOPE, "prompt-1", [file]);

    expect(listener).toHaveBeenCalledTimes(1);
    expect(listener).toHaveBeenCalledWith({
      files: [file],
      quote: null,
      text: "First line\nSecond line",
    });
  });

  it("Should restore the annotation and keep the quote identity off the editor text", () => {
    const recovery = new SessionPromptRecovery();
    const listener = vi.fn();
    const quote = buildTerminalQuote({
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["FAIL"],
    });
    recovery.subscribe(SCOPE, listener);

    recovery.stage(SCOPE, "prompt-1", {
      messages: [
        {
          id: "prompt-quote",
          parts: [{ text: composeQuotedPrompt("What failed?", quote), type: "text" }],
          role: "user",
        },
      ],
    });
    recovery.recover(SCOPE, "prompt-1", []);

    expect(listener).toHaveBeenCalledWith({
      files: [],
      quote,
      text: "What failed?",
    });
    expect(listener.mock.calls[0]?.[0].text).not.toContain("<terminal_context");
  });

  it("Should discard an accepted draft and unsubscribe listeners", () => {
    const recovery = new SessionPromptRecovery();
    const listener = vi.fn();
    const unsubscribe = recovery.subscribe(SCOPE, listener);

    recovery.stage(SCOPE, "prompt-1", { messages });
    recovery.acknowledge(SCOPE, "prompt-1");
    recovery.recover(SCOPE, "prompt-1", []);
    unsubscribe();
    recovery.stage(SCOPE, "prompt-1", { messages });
    recovery.recover(SCOPE, "prompt-1", []);

    expect(listener).not.toHaveBeenCalled();
  });

  it("Should recover a rejected prompt only in its originating workspace and session", () => {
    const recovery = new SessionPromptRecovery();
    const originalListener = vi.fn();
    const nextListener = vi.fn();
    const nextScope = { workspaceId: "workspace-atlas", sessionId: "session-next" };
    recovery.subscribe(SCOPE, originalListener);
    recovery.subscribe(nextScope, nextListener);

    recovery.stage(SCOPE, "prompt-1", { messages });
    recovery.stage(nextScope, "prompt-next", {
      messages: [
        {
          id: "prompt-next",
          parts: [{ text: "Next session", type: "text" }],
          role: "user",
        },
      ],
    });
    recovery.recover(SCOPE, "prompt-1", []);

    expect(originalListener).toHaveBeenCalledOnce();
    expect(nextListener).not.toHaveBeenCalled();

    recovery.recover(nextScope, "prompt-next", []);
    expect(nextListener).toHaveBeenCalledOnce();
    expect(nextListener).toHaveBeenCalledWith({ files: [], quote: null, text: "Next session" });
  });

  it("Should hand a rejection that landed with no composer to the session's next composer", () => {
    const recovery = new SessionPromptRecovery();
    const departed = vi.fn();
    const unsubscribe = recovery.subscribe(SCOPE, departed);
    const file = new File(["shot"], "shot.png", { type: "image/png" });

    recovery.stage(SCOPE, "prompt-1", { messages });
    unsubscribe();
    recovery.recover(SCOPE, "prompt-1", [file]);
    const returned = vi.fn();
    recovery.subscribe(SCOPE, returned);
    recovery.subscribe(SCOPE, vi.fn());

    expect(departed).not.toHaveBeenCalled();
    expect(returned).toHaveBeenCalledOnce();
    expect(returned).toHaveBeenCalledWith({
      files: [file],
      quote: null,
      text: "First line\nSecond line",
    });
  });

  it("Should restore a rejected draft into exactly one composer of its session", () => {
    const recovery = new SessionPromptRecovery();
    const first = vi.fn();
    const second = vi.fn();
    recovery.subscribe(SCOPE, first);
    recovery.subscribe(SCOPE, second);

    recovery.stage(SCOPE, "prompt-1", { messages });
    recovery.recover(SCOPE, "prompt-1", []);

    expect(first.mock.calls.length + second.mock.calls.length).toBe(1);
  });

  it("Should settle overlapping prompts of one session only by their own identity", () => {
    const recovery = new SessionPromptRecovery();
    const listener = vi.fn();
    const earlierFile = new File(["a"], "earlier.png", { type: "image/png" });
    const laterFile = new File(["b"], "later.png", { type: "image/png" });
    const prompt = (id: string, text: string) => ({
      messages: [{ id, parts: [{ text, type: "text" as const }], role: "user" as const }],
    });
    recovery.subscribe(SCOPE, listener);

    recovery.stage(SCOPE, "prompt-earlier", prompt("prompt-earlier", "Earlier"));
    recovery.stage(SCOPE, "prompt-later", prompt("prompt-later", "Later"));
    recovery.acknowledge(SCOPE, "prompt-earlier");
    recovery.recover(SCOPE, "prompt-later", [laterFile]);
    recovery.stage(SCOPE, "prompt-third", prompt("prompt-third", "Third"));
    recovery.recover(SCOPE, "prompt-earlier", [earlierFile]);

    expect(listener).toHaveBeenCalledOnce();
    expect(listener).toHaveBeenCalledWith({ files: [laterFile], quote: null, text: "Later" });
  });

  it("Should keep a delivered draft until a composer settles it", () => {
    const recovery = new SessionPromptRecovery();
    const departing = vi.fn();
    const unsubscribe = recovery.subscribe(SCOPE, departing);

    recovery.stage(SCOPE, "prompt-1", { messages });
    recovery.recover(SCOPE, "prompt-1", []);
    unsubscribe();
    const next = vi.fn();
    const unsubscribeNext = recovery.subscribe(SCOPE, next);
    const restored = next.mock.calls[0]?.[0];
    recovery.settle(SCOPE, restored);
    unsubscribeNext();
    const later = vi.fn();
    recovery.subscribe(SCOPE, later);

    expect(departing).toHaveBeenCalledOnce();
    expect(next).toHaveBeenCalledOnce();
    expect(restored).toEqual(departing.mock.calls[0]?.[0]);
    expect(later).not.toHaveBeenCalled();
  });
  it("Should hand a send lost with no page mounted to that session's next page only", () => {
    const recovery = new SessionPromptRecovery();
    const envelope = {
      action: "prompt" as const,
      attachments: [],
      expectedTurnId: null,
      identity: { idempotencyKey: "key-1", messageId: "prompt-1" },
      runtime: null,
      text: "Edited request",
    };
    recovery.retainUnconfirmed(SCOPE, envelope);

    const other = vi.fn();
    recovery.subscribeUnconfirmed({ ...SCOPE, sessionId: "session-other" }, other);
    const page = vi.fn();
    const unsubscribe = recovery.subscribeUnconfirmed(SCOPE, page);
    const later = vi.fn();
    recovery.subscribeUnconfirmed(SCOPE, later);

    expect(other).not.toHaveBeenCalled();
    expect(page).toHaveBeenCalledExactlyOnceWith(envelope);
    expect(later).not.toHaveBeenCalled();

    unsubscribe();
    const next = { ...envelope, identity: { idempotencyKey: "key-2", messageId: "prompt-2" } };
    recovery.retainUnconfirmed(SCOPE, next);
    expect(later).toHaveBeenCalledExactlyOnceWith(next);
    expect(page).toHaveBeenCalledTimes(1);
  });
});
