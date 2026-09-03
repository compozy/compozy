// Suite: session prompt recovery
// Invariant: only a locally rejected prompt restores its latest annotation,
// quote identity, and files once; the editor text never contains the envelope.
// Accepted prompts and unsubscribed listeners cannot restore a draft.
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

    recovery.stage(SCOPE, { messages });
    recovery.recover(SCOPE, [file]);
    recovery.recover(SCOPE, [file]);

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

    recovery.stage(SCOPE, {
      messages: [
        {
          id: "prompt-quote",
          parts: [{ text: composeQuotedPrompt("What failed?", quote), type: "text" }],
          role: "user",
        },
      ],
    });
    recovery.recover(SCOPE, []);

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

    recovery.stage(SCOPE, { messages });
    recovery.acknowledge(SCOPE);
    recovery.recover(SCOPE, []);
    unsubscribe();
    recovery.stage(SCOPE, { messages });
    recovery.recover(SCOPE, []);

    expect(listener).not.toHaveBeenCalled();
  });

  it("Should recover a rejected prompt only in its originating workspace and session", () => {
    const recovery = new SessionPromptRecovery();
    const originalListener = vi.fn();
    const nextListener = vi.fn();
    const nextScope = { workspaceId: "workspace-atlas", sessionId: "session-next" };
    recovery.subscribe(SCOPE, originalListener);
    recovery.subscribe(nextScope, nextListener);

    recovery.stage(SCOPE, { messages });
    recovery.stage(nextScope, {
      messages: [
        {
          id: "prompt-next",
          parts: [{ text: "Next session", type: "text" }],
          role: "user",
        },
      ],
    });
    recovery.recover(SCOPE, []);

    expect(originalListener).toHaveBeenCalledOnce();
    expect(nextListener).not.toHaveBeenCalled();

    recovery.recover(nextScope, []);
    expect(nextListener).toHaveBeenCalledOnce();
    expect(nextListener).toHaveBeenCalledWith({ files: [], quote: null, text: "Next session" });
  });
});
