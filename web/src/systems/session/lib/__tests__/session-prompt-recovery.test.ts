// Suite: session prompt recovery
// Invariant: only a locally rejected prompt restores its latest text and files
// once; accepted prompts and unsubscribed listeners cannot restore a draft.
// Owning layer: systems/session/lib prompt recovery lifecycle.
import { describe, expect, it, vi } from "vitest";

import { SessionPromptRecovery } from "../session-prompt-recovery";

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
    recovery.subscribe(listener);

    recovery.stage({ messages });
    recovery.recover([file]);
    recovery.recover([file]);

    expect(listener).toHaveBeenCalledTimes(1);
    expect(listener).toHaveBeenCalledWith({ files: [file], text: "First line\nSecond line" });
  });

  it("Should discard an accepted draft and unsubscribe listeners", () => {
    const recovery = new SessionPromptRecovery();
    const listener = vi.fn();
    const unsubscribe = recovery.subscribe(listener);

    recovery.stage({ messages });
    recovery.acknowledge();
    recovery.recover([]);
    unsubscribe();
    recovery.stage({ messages });
    recovery.recover([]);

    expect(listener).not.toHaveBeenCalled();
  });
});
