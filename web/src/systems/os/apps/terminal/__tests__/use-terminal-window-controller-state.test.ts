// Suite: OS terminal-window controller host gates
// Invariant: the journal query stays disabled until first reveal, and the
// unlock survives window remounts without crossing workspaces.
// Boundary IN: journal enable gate used by the host.
// Boundary OUT: recording stop reconcile and elapsed timer, owned by
// terminal-recording-state / use-terminal-recordings.
// Close invariant: only confirmed running targets terminate; cancellation,
// stale scope, or failed termination keeps the managed window retryable.

import { QueryClient } from "@tanstack/react-query";
import { waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { toast } from "@compozy/ui";
import { closeTerminal, fetchTerminals, TerminalApiError } from "@/systems/terminal";
import { DEV_SERVER_TERMINAL } from "@/systems/terminal/mocks/terminal-fixtures";
import { TerminalWindowClose, terminalWindowCreateKey } from "../../../lib/terminal-window-close";
import type { OsWindow } from "../../../lib/os-types";

vi.mock("@/systems/terminal/adapters/terminal-api", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/terminal/adapters/terminal-api")>();
  return { ...actual, fetchTerminals: vi.fn(), closeTerminal: vi.fn() };
});

const terminalWindow: OsWindow = {
  id: "window:terminal",
  app: "terminal",
  instanceKey: DEV_SERVER_TERMINAL.id,
  route: { pathname: `/terminal/${DEV_SERVER_TERMINAL.id}`, search: {} },
  navStack: [],
  pinned: false,
  desktopId: "desktop:one",
  placement: "floating",
  rect: { x: 0, y: 0, w: 600, h: 400 },
  layer: 1,
  minimized: false,
  zoomed: false,
  groupId: null,
  nodeId: null,
  stackId: null,
  stackActive: true,
  parentAxis: null,
};
const terminalExit = {
  at: "2026-09-10T17:00:00Z",
  cause: "signaled" as const,
  code: null,
  signal: "HUP" as const,
};

beforeEach(() => {
  vi.mocked(fetchTerminals).mockReset().mockResolvedValue([DEV_SERVER_TERMINAL]);
  vi.mocked(closeTerminal).mockReset().mockResolvedValue(terminalExit);
  vi.spyOn(toast, "error").mockImplementation(() => "toast");
});

import { terminalJournalQueryEnabled } from "../lib/terminal-window-journal";
import { terminalJournalUnlockLogic } from "@/systems/terminal/stores/terminal-journal-unlock-store";

describe("useTerminalWindowControllerState journal and recording host gates", () => {
  it("Should keep the journal query disabled until the journal is first opened", () => {
    expect(terminalJournalQueryEnabled("", false)).toBe(false);
    expect(terminalJournalQueryEnabled("ws-atlas", false)).toBe(false);
    expect(terminalJournalQueryEnabled("ws-atlas", true)).toBe(true);
  });

  it("Should preserve first-open state across window remounts without crossing workspaces", () => {
    const store = terminalJournalUnlockLogic.createStore();

    store.trigger.journalOpened({ workspaceId: "ws-atlas" });

    expect(store.getSnapshot().context.unlockedWorkspaces).toEqual({ "ws-atlas": true });
    expect(store.getSnapshot().context.unlockedWorkspaces["ws-other"]).toBeUndefined();
  });
});

describe("terminal window close lifecycle", () => {
  function setup() {
    const controller = new TerminalWindowClose();
    const guard = controller.guard("ws-close", new QueryClient(), "default");
    return { controller, guard };
  }

  it("Should leave process and window intact when confirmation is canceled", async () => {
    const { controller, guard } = setup();
    const result = guard([terminalWindow], () => true);
    await waitFor(() => expect(controller.confirmation.get()).not.toBeNull());
    controller.cancel();
    await expect(result).resolves.toBe(false);
    expect(closeTerminal).not.toHaveBeenCalled();
  });

  it("Should close only selected running terminals using their exact workspace and profile", async () => {
    const other = { ...DEV_SERVER_TERMINAL, id: "unrelated" };
    vi.mocked(fetchTerminals).mockResolvedValue([DEV_SERVER_TERMINAL, other]);
    const { controller, guard } = setup();
    const result = guard([terminalWindow], () => true);
    await waitFor(() =>
      expect(controller.confirmation.get()?.terminals).toEqual([DEV_SERVER_TERMINAL])
    );
    controller.confirmation.get()?.answer(true);
    await expect(result).resolves.toBe(true);
    expect(closeTerminal).toHaveBeenCalledExactlyOnceWith(
      "ws-close",
      DEV_SERVER_TERMINAL.id,
      { profile: DEV_SERVER_TERMINAL.profile_name },
      "HUP",
      expect.any(AbortSignal)
    );
  });

  it("Should retain a pending launcher and allow closing it after creation fails", async () => {
    const client = new QueryClient();
    const controller = new TerminalWindowClose();
    let fail!: (error: Error) => void;
    const mutation = client.getMutationCache().build(client, {
      mutationKey: terminalWindowCreateKey("ws-close", "default", terminalWindow.id),
      mutationFn: () =>
        new Promise((_resolve, reject) => {
          fail = reject;
        }),
    });
    const creation = mutation.execute(undefined).catch(() => undefined);
    await waitFor(() => expect(fail).toBeDefined());
    const guard = controller.guard("ws-close", client, "default");
    const launcher = { ...terminalWindow, instanceKey: null };
    await expect(guard([launcher], () => true)).resolves.toBe(false);
    fail(new Error("create unavailable"));
    await creation;
    await expect(guard([launcher], () => true)).resolves.toBe(true);
    expect(closeTerminal).not.toHaveBeenCalled();
  });

  it.each(["exited", "missing"])(
    "Should close an %s terminal window without warning",
    async state => {
      vi.mocked(fetchTerminals).mockResolvedValue(
        state === "missing" ? [] : [{ ...DEV_SERVER_TERMINAL, state: "exited" }]
      );
      const { controller, guard } = setup();
      await expect(guard([terminalWindow], () => true)).resolves.toBe(true);
      expect(controller.confirmation.get()).toBeNull();
      expect(closeTerminal).not.toHaveBeenCalled();
    }
  );

  it("Should re-read a terminal that exits while the dialog is open", async () => {
    const { controller, guard } = setup();
    const result = guard([terminalWindow], () => true);
    await waitFor(() => expect(controller.confirmation.get()).not.toBeNull());
    vi.mocked(fetchTerminals).mockResolvedValue([{ ...DEV_SERVER_TERMINAL, state: "exited" }]);
    controller.confirmation.get()?.answer(true);
    await expect(result).resolves.toBe(true);
    expect(closeTerminal).not.toHaveBeenCalled();
  });

  it("Should abandon termination after the window or workspace changes", async () => {
    let current = true;
    const { controller, guard } = setup();
    const result = guard([terminalWindow], () => current);
    await waitFor(() => expect(controller.confirmation.get()).not.toBeNull());
    current = false;
    controller.confirmation.get()?.answer(true);
    await expect(result).resolves.toBe(false);
    expect(closeTerminal).not.toHaveBeenCalled();
  });

  it.each([
    new Error("close refused"),
    new TerminalApiError("unavailable", 503, "service_unavailable"),
  ])("Should surface termination failure and allow a fresh retry", async error => {
    const { controller, guard } = setup();
    vi.mocked(closeTerminal).mockRejectedValueOnce(error);
    const result = guard([terminalWindow], () => true);
    await waitFor(() => expect(controller.confirmation.get()).not.toBeNull());
    controller.confirmation.get()?.answer(true);
    await expect(result).resolves.toBe(false);
    expect(toast.error).toHaveBeenCalledWith("Could not close terminal", expect.any(Object));
    const retry = guard([terminalWindow], () => true);
    await waitFor(() => expect(controller.confirmation.get()).not.toBeNull());
    controller.confirmation.get()?.answer(true);
    await expect(retry).resolves.toBe(true);
  });

  it("Should tolerate a terminal removed between the final read and close", async () => {
    vi.mocked(closeTerminal).mockRejectedValue(
      new TerminalApiError("missing", 404, "terminal_not_found")
    );
    const { controller, guard } = setup();
    const result = guard([terminalWindow], () => true);
    await waitFor(() => expect(controller.confirmation.get()).not.toBeNull());
    controller.confirmation.get()?.answer(true);
    await expect(result).resolves.toBe(true);
  });

  it("Should preserve a window when the daemon cannot prove its terminal state", async () => {
    vi.mocked(fetchTerminals).mockRejectedValue(new Error("offline"));
    const { guard } = setup();
    await expect(guard([terminalWindow], () => true)).resolves.toBe(false);
    expect(closeTerminal).not.toHaveBeenCalled();
  });

  it("Should close other apps without touching the terminal API", async () => {
    const { guard } = setup();
    await expect(guard([{ ...terminalWindow, app: "tasks" }], () => true)).resolves.toBe(true);
    expect(fetchTerminals).not.toHaveBeenCalled();
  });
});
