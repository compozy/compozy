// Suite: OS terminal-window controller host gates
// Invariant: the journal query stays disabled until first reveal, and the
// unlock survives window remounts without crossing workspaces.
// Boundary IN: journal enable gate used by the host.
// Boundary OUT: recording stop reconcile and elapsed timer, owned by
// terminal-recording-state / use-terminal-recordings.
// Close invariant: only confirmed running targets terminate; cancellation,
// stale scope, or failed termination keeps the managed window retryable.

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { toast } from "@compozy/ui";
import {
  closeTerminal,
  createTerminal,
  fetchTerminals,
  terminalKeys,
  terminalScope,
  TerminalApiError,
  unlockTerminalJournal,
} from "@/systems/terminal";
import {
  fetchTerminalInputRequestProjection,
  fetchTerminalJournal,
  fetchTerminalRecording,
} from "@/systems/terminal/adapters/terminal-api";
import { DEV_SERVER_TERMINAL } from "@/systems/terminal/mocks/terminal-fixtures";
import { TerminalWindowClose, terminalWindowCreateKey } from "../../../lib/terminal-window-close";
import type { OsWindow } from "../../../lib/os-types";
import { executeWindowManagerCommand } from "../../../adapters/window-manager-api";
import { RoutingCoordinator } from "../../../lib/routing-coordinator";
import { windowManagerKeys } from "../../../lib/window-manager-query";
import {
  parseWindowManagerSnapshot,
  parseWindowManagerClientView,
} from "../../../lib/window-manager-schemas";
import { parseSettingsWindowManagerSection } from "../../../lib/window-manager-settings-section";
import { windowManagerClientFixture, windowManagerStorySnapshot } from "../../../mocks/fixtures";
import { settingsWindowManagerSectionFixture } from "@/systems/settings/mocks/window-manager-fixtures";
import { latchGatewayTierForTest } from "@/test/gateway-tier";
import { WindowManagerRuntime } from "../../../runtime/window-manager-runtime";
import { useTerminalWindowCreation } from "../hooks/use-terminal-window-creation";
import { useTerminalWindowControllerState } from "../hooks/use-terminal-window-controller-state";

vi.mock("@/systems/terminal/adapters/terminal-api", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/terminal/adapters/terminal-api")>();
  return {
    ...actual,
    fetchTerminals: vi.fn(),
    closeTerminal: vi.fn(),
    createTerminal: vi.fn(),
    fetchTerminalInputRequestProjection: vi.fn(),
    fetchTerminalJournal: vi.fn(),
    fetchTerminalRecording: vi.fn(),
  };
});

// The controller host renders inside the OS shell; these suite-local handles
// keep the hook composition testable without a full desktop provider tree.
vi.mock("../../../hooks/use-os-shell", () => ({
  useOsShell: () => ({ coordinator: {}, manager: {} }),
}));
vi.mock("../../../hooks/use-desktop", () => ({
  // A cold desktop: no managed window, no attached client identity.
  useDesktop: (selector: (state: unknown) => unknown) =>
    selector({ windows: {}, client: null, clientAttachmentToken: null }),
}));
vi.mock("@/systems/terminal/hooks/use-terminal-catalog-stream", async importOriginal => {
  const actual =
    await importOriginal<typeof import("@/systems/terminal/hooks/use-terminal-catalog-stream")>();
  return { ...actual, useTerminalCatalogStream: vi.fn(() => undefined) };
});
vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: vi.fn(() => ({ runtimeWorkspaceId: "ws-atlas" })),
}));
vi.mock("@/systems/profiles/hooks/use-profile-read-scope", () => ({
  useProfileReadScope: vi.fn(() => ({
    destination: "work",
    aggregate: false,
    destinationOwner: { id: "profile-work" },
  })),
}));
vi.mock("@/systems/profiles/hooks/use-profiles", () => ({
  useProfiles: vi.fn(() => ({ data: [] })),
}));
vi.mock("@/systems/settings/hooks/use-settings-sections", async importOriginal => {
  const actual =
    await importOriginal<typeof import("@/systems/settings/hooks/use-settings-sections")>();
  return { ...actual, useSettingsGeneral: vi.fn(() => ({})) };
});

vi.mock("../../../adapters/window-manager-api", async importOriginal => {
  const actual = await importOriginal<typeof import("../../../adapters/window-manager-api")>();
  return { ...actual, executeWindowManagerCommand: vi.fn() };
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

let unlatchGatewayTier: () => void;

beforeEach(() => {
  // Terminal reads and the window surface render on the local tier only.
  unlatchGatewayTier = latchGatewayTierForTest("local");
  vi.mocked(createTerminal).mockReset();
  vi.mocked(executeWindowManagerCommand).mockReset();
  vi.mocked(fetchTerminals).mockReset().mockResolvedValue([DEV_SERVER_TERMINAL]);
  vi.mocked(closeTerminal).mockReset().mockResolvedValue(terminalExit);
  vi.spyOn(toast, "error")
    .mockClear()
    .mockImplementation(() => "toast");
});

afterEach(() => {
  unlatchGatewayTier();
});

// Invariant: terminal creation completes in its initiating query scope and cannot
// retarget a desktop after rebinding; completed creation remains recoverable
// after host remount. Owner: this terminal controller host suite.
describe("terminal creation scope", () => {
  it.each(["unchanged", "workspace", "profile", "unbound", "return"] as const)(
    "Should preserve the initiating terminal when the shell scope is %s",
    async change => {
      const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
      const runtime = new WindowManagerRuntime(client);
      const push = vi.fn();
      const coordinator = new RoutingCoordinator(runtime, { navigate: push, replace: vi.fn() });
      const windowId = "w-story-terminal";
      const original = terminalScope("ws-atlas", "work");
      const other = terminalScope(change === "workspace" ? "ws-other" : "ws-atlas", "other");
      const readKeys = (scope: typeof original) => [
        terminalKeys.catalog(scope.key),
        terminalKeys.inputRequests(scope.key),
        terminalKeys.journalScope(scope.key),
      ];
      for (const key of [...readKeys(original), ...readKeys(other)]) client.setQueryData(key, []);
      const bind = (scope: typeof original) => {
        const snapshot = parseWindowManagerSnapshot(
          windowManagerStorySnapshot("/terminal", scope.key.workspaceId)
        );
        client.setQueryData(
          windowManagerKeys.snapshot(scope.key.workspaceId, scope.key.profileKey),
          snapshot
        );
        client.setQueryData(
          windowManagerKeys.config(scope.key.workspaceId, "client:web"),
          parseSettingsWindowManagerSection(settingsWindowManagerSectionFixture)
        );
        runtime.bind({
          workspaceId: scope.key.workspaceId,
          profileId: scope.key.profileKey,
          clientId: "client:web",
        });
        runtime.setClient(
          parseWindowManagerClientView(
            windowManagerClientFixture("client:web", scope.key.workspaceId, windowId)
          )
        );
        return snapshot;
      };
      const snapshot = bind(original);
      runtime.start();
      vi.mocked(executeWindowManagerCommand).mockResolvedValue({
        snapshot,
        applied: true,
        client: null,
        diagnostics: [],
        rebasedFrom: null,
        changes: {
          desktopIds: [],
          windowIds: [],
          groupIds: [],
          nodeIds: [],
          clientIds: [],
          stackGrouped: [],
          stackUngrouped: [],
        },
      });
      let resolve!: (terminal: typeof DEV_SERVER_TERMINAL) => void;
      vi.mocked(createTerminal).mockReturnValue(
        new Promise(done => {
          resolve = done;
        })
      );
      const renderCreation = (initialScope: typeof original) =>
        renderHook(
          scope =>
            useTerminalWindowCreation({
              windowId,
              workspaceId: scope.key.workspaceId,
              catalogScope: scope,
              destinationScope: scope,
              coordinator,
            }),
          {
            initialProps: initialScope,
            wrapper: ({ children }: { children: ReactNode }) =>
              createElement(QueryClientProvider, { client }, children),
          }
        );
      const { result, rerender, unmount } = renderCreation(original);
      try {
        const identity = { id: "client:web", attachmentToken: "viewer-token" };
        let creation!: Promise<typeof DEV_SERVER_TERMINAL>;
        act(() => {
          creation = result.current.mutateAsync(identity);
        });
        await waitFor(() =>
          expect(createTerminal).toHaveBeenCalledExactlyOnceWith(
            "ws-atlas",
            {},
            { profile: "work" },
            identity
          )
        );
        expect(result.current.isPending).toBe(true);
        if (change !== "unchanged") {
          act(() => {
            if (change === "unbound") runtime.unbind();
            else bind(other);
          });
          rerender(other);
          if (change === "return") {
            act(() => {
              bind(original);
            });
            rerender(original);
          }
        }
        await act(async () => {
          resolve(DEV_SERVER_TERMINAL);
          await creation;
        });
        for (const key of readKeys(original))
          expect(client.getQueryState(key)?.isInvalidated).toBe(true);
        for (const key of readKeys(other))
          expect(client.getQueryState(key)?.isInvalidated).toBe(false);
        if (change === "unchanged") {
          expect(executeWindowManagerCommand).toHaveBeenCalledExactlyOnceWith(
            "ws-atlas",
            "work",
            "client:web",
            snapshot.revision,
            {
              commandId: "window.navigate",
              payload: {
                window_id: windowId,
                instance_key: DEV_SERVER_TERMINAL.id,
                route: { pathname: `/terminal/${DEV_SERVER_TERMINAL.id}`, search: {} },
              },
            }
          );
          expect(push).toHaveBeenCalledOnce();
        } else {
          expect(executeWindowManagerCommand).not.toHaveBeenCalled();
          expect(push).not.toHaveBeenCalled();
        }
        await waitFor(() => expect(result.current.isPending).toBe(false));
        expect(result.current.completedTerminal).toEqual(
          change === "unchanged" || change === "return" ? DEV_SERVER_TERMINAL : null
        );
        if (change !== "unchanged" && change !== "return") {
          act(() => {
            bind(original);
          });
          rerender(original);
          expect(result.current.completedTerminal).toEqual(DEV_SERVER_TERMINAL);
          expect(executeWindowManagerCommand).not.toHaveBeenCalled();
        }
        if (change === "workspace") {
          unmount();
          const remounted = renderCreation(original);
          try {
            expect(remounted.result.current.completedTerminal).toEqual(DEV_SERVER_TERMINAL);
            expect(createTerminal).toHaveBeenCalledOnce();
            expect(executeWindowManagerCommand).not.toHaveBeenCalled();
          } finally {
            remounted.unmount();
          }
        }
      } finally {
        unmount();
        runtime.unbind();
        runtime.stop();
        client.clear();
      }
    }
  );
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

  const renderController = () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const rendered = renderHook(() => useTerminalWindowControllerState("window:terminal"), {
      wrapper: ({ children }: { children: ReactNode }) =>
        createElement(QueryClientProvider, { client }, children),
    });
    return { ...rendered, client };
  };

  it("Should not fetch journal or recording reads on a remote tier", async () => {
    // Regression: an unlocked journal (or a selected replay) enabled its own
    // query regardless of tier, firing requests at terminal routes a remote
    // tier never registers before the window renders its loopback-only state.
    unlatchGatewayTier = latchGatewayTierForTest("private");
    unlockTerminalJournal("ws-atlas");
    vi.mocked(fetchTerminalInputRequestProjection).mockReset();
    vi.mocked(fetchTerminalJournal).mockReset();
    vi.mocked(fetchTerminalRecording).mockReset();

    const { result, unmount, client } = renderController();
    try {
      act(() => {
        result.current.setReplay({ id: "rec-1", profile: "work", title: "Replay" });
      });
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 20));
      });

      expect(fetchTerminals).not.toHaveBeenCalled();
      expect(fetchTerminalInputRequestProjection).not.toHaveBeenCalled();
      expect(fetchTerminalJournal).not.toHaveBeenCalled();
      expect(fetchTerminalRecording).not.toHaveBeenCalled();
    } finally {
      unmount();
      client.clear();
    }
  });

  it("Should fetch the journal and a selected replay on the local tier", async () => {
    unlockTerminalJournal("ws-atlas");
    vi.mocked(fetchTerminalInputRequestProjection)
      .mockReset()
      .mockResolvedValue({ pending: [], resolved: [] } as never);
    vi.mocked(fetchTerminalJournal)
      .mockReset()
      .mockResolvedValue({ entries: [], next: null } as never);
    vi.mocked(fetchTerminalRecording)
      .mockReset()
      .mockResolvedValue({} as never);

    const { result, unmount, client } = renderController();
    try {
      await waitFor(() => expect(fetchTerminalJournal).toHaveBeenCalled());
      expect(vi.mocked(fetchTerminalJournal).mock.calls[0]?.[0]).toBe("ws-atlas");

      act(() => {
        result.current.setReplay({ id: "rec-1", profile: "work", title: "Replay" });
      });
      await waitFor(() =>
        expect(fetchTerminalRecording).toHaveBeenCalledExactlyOnceWith(
          "ws-atlas",
          "rec-1",
          { profile: "work" },
          expect.anything()
        )
      );
    } finally {
      unmount();
      client.clear();
    }
  });
});

describe("terminal window close lifecycle", () => {
  function setup() {
    const controller = new TerminalWindowClose();
    const guard = controller.guard("ws-close", new QueryClient());
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

  it.each(["ws-other", null])(
    "Should retain pending creation when rebinding to workspace %s",
    async workspaceId => {
      const client = new QueryClient();
      const controller = new TerminalWindowClose();
      let fail!: (error: Error) => void;
      const mutation = client.getMutationCache().build(client, {
        mutationKey: terminalWindowCreateKey(terminalWindow.id),
        mutationFn: () =>
          new Promise((_resolve, reject) => {
            fail = reject;
          }),
      });
      const creation = mutation.execute(undefined).catch(() => undefined);
      await waitFor(() => expect(fail).toBeDefined());
      const guard = controller.guard("ws-close", client);
      const launcher = { ...terminalWindow, instanceKey: null };
      await expect(guard([launcher], () => true)).resolves.toBe(false);
      // The launcher keeps its identity while workspace/profile selection changes.
      // Its initiating mutation must remain visible to the new shell guard.
      controller.cancel();
      const rebound = controller.guard(workspaceId, client);
      await expect(rebound([launcher], () => true)).resolves.toBe(false);
      await expect(rebound([{ ...launcher, id: "another-window" }], () => true)).resolves.toBe(
        true
      );
      fail(new Error("create unavailable"));
      await creation;
      await expect(rebound([launcher], () => true)).resolves.toBe(true);
      expect(closeTerminal).not.toHaveBeenCalled();
    }
  );

  it("Should await all concurrent closes after partial failure before allowing a fresh retry", async () => {
    const second = { ...DEV_SERVER_TERMINAL, id: "second", profile_name: "second-profile" };
    vi.mocked(fetchTerminals).mockResolvedValue([DEV_SERVER_TERMINAL, second]);
    let rejectFirst!: (error: Error) => void;
    let resolveSecond!: (exit: typeof terminalExit) => void;
    vi.mocked(closeTerminal)
      .mockReturnValueOnce(
        new Promise((_resolve, reject) => {
          rejectFirst = reject;
        })
      )
      .mockReturnValueOnce(
        new Promise(resolve => {
          resolveSecond = resolve;
        })
      );
    const { controller, guard } = setup();
    const windows = [
      terminalWindow,
      { ...terminalWindow, id: "window:second", instanceKey: second.id },
    ];
    const settled = vi.fn();
    const result = guard(windows, () => true).then(settled);
    await waitFor(() => expect(controller.confirmation.get()).not.toBeNull());
    controller.confirmation.get()?.answer(true);
    await waitFor(() => expect(closeTerminal).toHaveBeenCalledTimes(2));
    await act(async () => {
      rejectFirst(new Error("first close refused"));
    });
    expect(settled).not.toHaveBeenCalled();
    expect(toast.error).not.toHaveBeenCalled();
    resolveSecond(terminalExit);
    await result;
    expect(settled).toHaveBeenCalledExactlyOnceWith(false);
    expect(toast.error).toHaveBeenCalledWith("Could not close terminal", {
      description: "first close refused",
    });

    vi.mocked(fetchTerminals).mockResolvedValue([
      DEV_SERVER_TERMINAL,
      { ...second, state: "exited" },
    ]);
    const retry = guard(windows, () => true);
    await waitFor(() =>
      expect(controller.confirmation.get()?.terminals).toEqual([DEV_SERVER_TERMINAL])
    );
    controller.confirmation.get()?.answer(true);
    await expect(retry).resolves.toBe(true);
    expect(closeTerminal).toHaveBeenCalledTimes(3);
    expect(closeTerminal).toHaveBeenLastCalledWith(
      "ws-close",
      DEV_SERVER_TERMINAL.id,
      { profile: DEV_SERVER_TERMINAL.profile_name },
      "HUP",
      expect.any(AbortSignal)
    );
  });

  it("Should reject the complete batch before terminating any unconfirmed profile", async () => {
    const second = { ...DEV_SERVER_TERMINAL, id: "second" };
    vi.mocked(fetchTerminals).mockResolvedValue([DEV_SERVER_TERMINAL, second]);
    const { controller, guard } = setup();
    const result = guard(
      [terminalWindow, { ...terminalWindow, id: "window:second", instanceKey: second.id }],
      () => true
    );
    await waitFor(() => expect(controller.confirmation.get()).not.toBeNull());
    vi.mocked(fetchTerminals).mockResolvedValue([
      DEV_SERVER_TERMINAL,
      { ...second, profile_name: "changed" },
    ]);
    controller.confirmation.get()?.answer(true);
    await expect(result).resolves.toBe(false);
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
