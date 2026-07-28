import { describe, expect, it, vi } from "vitest";

vi.mock("sonner", () => ({ toast: { success: vi.fn() } }));

import { toast } from "sonner";

import { emptyDraft } from "@/systems/settings";
import { marketplaceMCPEditorLogic } from "../marketplace-mcp-editor-logic";

describe("marketplace MCP editor logic", () => {
  it("Should keep the editor mounted until its accepted save settles", () => {
    const store = marketplaceMCPEditorLogic.createStore();
    vi.mocked(toast.success).mockClear();

    store.trigger.editorOpened({
      editor: {
        draft: emptyDraft("stdio"),
        mode: "create",
        scope: "global",
        target: "auto",
      },
    });
    store.trigger.saveRequested({
      execute: () => new Promise(() => undefined),
      name: "local",
    });
    const attempt = store.getSnapshot().context.pendingSaveAttempt;
    store.trigger.editorDismissed();
    expect(store.getSnapshot().context.editor.mode).toBe("create");
    store.trigger.saveSucceeded({
      attempt: attempt ?? 0,
      name: "local",
      result: {} as never,
    });

    expect(store.getSnapshot().context.editor).toEqual({ mode: "closed" });
    expect(toast.success).toHaveBeenCalledWith(
      'Saved "local" · write target unavailable · applied now'
    );
  });
});
