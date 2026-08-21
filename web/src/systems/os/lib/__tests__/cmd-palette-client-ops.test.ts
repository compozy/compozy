// Suite: cmd-palette client-op lookup table
// Invariant: the table resolves tab, focus, desktop and layout ids to exactly
// one controller or shell call, and merge-all never includes minimized peers.
// Owning layer: web/src/systems/os/lib/cmd-palette-client-ops.ts
// Canonical suite: this file (split from the dispatch seam suite).
import { describe, expect, it, vi } from "vitest";

import { paletteClientOp } from "../cmd-palette-client-ops";
import { frameFixture, opContext } from "./cmd-palette-dispatch-fixtures";

function run(op: string, context: ReturnType<typeof opContext>, payload: unknown = {}) {
  const handler = paletteClientOp(op);
  if (handler === null) throw new Error(`no handler for ${op}`);
  return handler(
    {
      manager: context.manager,
      shell: context.shell,
      navigate: context.navigate,
      openUrl: context.openUrl,
    },
    payload
  );
}

describe("cmd-palette client-op table — tabs", () => {
  it("Should jump positionally, wrap next/previous from the active tab, and land last [UT-050]", () => {
    const frame = frameFixture();
    const jump = opContext(frame, "w-2");
    void run("window.tab.jump.3", jump);
    expect(jump.shell.activateWindow).toHaveBeenCalledExactlyOnceWith("w-3");

    const next = opContext(frame, "w-2");
    void run("window.tab.next", next);
    expect(next.shell.activateWindow).toHaveBeenCalledExactlyOnceWith("w-3");

    const wrap = opContext(frameFixture({ activeWindowId: "w-3" }), "w-3");
    void run("window.tab.next", wrap);
    expect(wrap.shell.activateWindow).toHaveBeenCalledExactlyOnceWith("w-1");

    const previous = opContext(frameFixture({ activeWindowId: "w-1" }), "w-1");
    void run("window.tab.previous", previous);
    expect(previous.shell.activateWindow).toHaveBeenCalledExactlyOnceWith("w-3");

    const last = opContext(frame, "w-2");
    void run("window.tab.last", last);
    expect(last.shell.activateWindow).toHaveBeenCalledExactlyOnceWith("w-3");
  });

  it("Should no-op jumps and cycling on a single-tab frame while ⌘T still opens the deck path [UT-052]", () => {
    const solo = frameFixture({
      id: "w-1",
      members: ["w-1"],
      activeWindowId: "w-1",
      stackId: null,
    });
    for (const op of [
      "window.tab.next",
      "window.tab.previous",
      "window.tab.last",
      "window.tab.jump.2",
    ]) {
      const context = opContext(solo, "w-1");
      void run(op, context);
      expect(context.shell.activateWindow).not.toHaveBeenCalled();
    }

    const newTab = opContext(solo, "w-1");
    void run("window.tab.new", newTab);
    expect(newTab.shell.openNewTab).toHaveBeenCalledExactlyOnceWith("w-1");
  });

  it("Should open a standalone new tab when no window is focused [UT-082]", () => {
    const context = opContext(null, null);
    void run("window.tab.new", context);
    expect(context.shell.openNewTab).toHaveBeenCalledExactlyOnceWith(null);
  });

  it("Should dispatch reopen without requiring a focused window [UT-052]", () => {
    const context = opContext(null, null);
    void run("window.tab.reopen", context);
    expect(context.manager.reopenWindow).toHaveBeenCalledOnce();
  });

  it("Should ignore a jump slot beyond the frame's members", () => {
    const context = opContext(frameFixture(), "w-2");
    void run("window.tab.jump.8", context);
    expect(context.shell.activateWindow).not.toHaveBeenCalled();
  });
});

describe("cmd-palette client-op table — navigation v2", () => {
  it("Should focus the previous MRU window and ignore minimized entries [UT-072]", () => {
    const context = opContext(null, "w-2");
    context.state.client.focusOrder = ["w-2", "w-3", "w-1"];
    context.state.windows["w-3"]!.minimized = true;

    void run("window.focus.last", context);

    expect(context.shell.activateWindow).toHaveBeenCalledExactlyOnceWith("w-1");
  });

  it("Should share stable desktop order with numeric switching and no-op missing slots [UT-073]", () => {
    const context = opContext(null, null);
    context.state.desktops = [
      { id: "desktop:c", order: 1 },
      { id: "desktop:b", order: 0 },
      { id: "desktop:a", order: 0 },
    ];

    void run("desktop.switch.2", context);
    expect(context.manager.switchDesktop).toHaveBeenCalledExactlyOnceWith("desktop:b");

    void run("desktop.switch.9", context);
    expect(context.manager.switchDesktop).toHaveBeenCalledOnce();
  });

  it("Should route session and attention actions through their shared shell owners [UT-074]", () => {
    const context = opContext(null, null);
    void run("session.cycle.next", context);
    void run("session.focus.attention", context);

    expect(context.shell.cycleSession).toHaveBeenCalledExactlyOnceWith("next");
    expect(context.shell.focusAttention).toHaveBeenCalledOnce();
  });
});

describe("cmd-palette client-op table — absorbed palette rows", () => {
  it("Should merge every visible peer into the focused window's frame", () => {
    const context = opContext(frameFixture(), "w-2");
    void run("window.merge_all", context);
    expect(context.manager.groupWindows).toHaveBeenCalledExactlyOnceWith("w-2", ["w-1", "w-3"]);
  });

  it("Should omit minimized peers from merge-all grouping", () => {
    const context = opContext(frameFixture(), "w-2");
    context.state.windows["w-3"]!.minimized = true;
    void run("window.merge_all", context);
    expect(context.manager.groupWindows).toHaveBeenCalledExactlyOnceWith("w-2", ["w-1"]);
  });

  it("Should float the focused tab out of its stack and follow it", async () => {
    const context = opContext(frameFixture(), "w-2");
    void run("window.tab.detach", context);
    expect(context.manager.toggleFloating).toHaveBeenCalledExactlyOnceWith("w-2");
    await vi.waitFor(() =>
      expect(context.shell.activateWindow).toHaveBeenCalledExactlyOnceWith("w-2")
    );
  });

  it("Should leave a lone window in place rather than detaching nothing", () => {
    const solo = frameFixture({ members: ["w-2"], activeWindowId: "w-2", stackId: null });
    const context = opContext(solo, "w-2");
    void run("window.tab.detach", context);
    expect(context.manager.toggleFloating).not.toHaveBeenCalled();
  });

  it("Should raise the sessions catalog through its shell owner", () => {
    const context = opContext(null, null);
    void run("shell.sessions.toggle", context);
    expect(context.shell.toggleSessions).toHaveBeenCalledOnce();
  });
});

describe("cmd-palette client-op table — daemon view/navigate/url", () => {
  it("Should open a palette view from the daemon view.open payload [RD0022]", () => {
    const context = opContext(null, null);
    void run("view.open", context, {
      action: { kind: "view", view: "sessions" },
      args: {},
    });
    expect(context.shell.openPaletteView).toHaveBeenCalledExactlyOnceWith("sessions");
  });

  it("Should navigate through the same port local dispatch uses [RD0022]", () => {
    const context = opContext(null, null);
    void run("navigate", context, {
      action: { kind: "navigate", app: "tasks" },
      args: { pathname: "/tasks" },
    });
    expect(context.navigate).toHaveBeenCalledExactlyOnceWith("tasks", "/tasks");
  });

  it("Should open a URL through the same port local dispatch uses [RD0022]", () => {
    const context = opContext(null, null);
    void run("url.open", context, {
      action: { kind: "url", url: "https://example.com/docs" },
      args: {},
    });
    expect(context.openUrl).toHaveBeenCalledExactlyOnceWith("https://example.com/docs");
  });

  it("Should refuse malformed daemon payloads for view, navigate, and url [RD0022]", () => {
    const context = opContext(null, null);
    expect(() => run("view.open", context, { action: { kind: "view" } })).toThrow(
      "malformed view.open payload"
    );
    expect(() => run("navigate", context, { action: { kind: "tool", app: "tasks" } })).toThrow(
      "malformed navigate payload"
    );
    expect(() => run("url.open", context, null)).toThrow("malformed url.open payload");
    expect(context.shell.openPaletteView).not.toHaveBeenCalled();
    expect(context.navigate).not.toHaveBeenCalled();
    expect(context.openUrl).not.toHaveBeenCalled();
  });
});
