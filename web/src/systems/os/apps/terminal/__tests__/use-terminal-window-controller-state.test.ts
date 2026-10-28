// Suite: OS terminal-window controller host gates
// Invariant: the journal query stays disabled until first reveal, and the
// unlock survives window remounts without crossing workspaces.
// Boundary IN: journal enable gate used by the host.
// Boundary OUT: recording stop reconcile and elapsed timer, owned by
// terminal-recording-state / use-terminal-recordings.
// Close invariant: only confirmed running targets terminate; cancellation,
// stale scope, or failed termination keeps the managed window retryable.
// All projects invariant: an unbound window lists the aggregate and opens in place, bound.

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { toast } from "@compozy/ui";
import {
  closeTerminal,
  createTerminal,
  fetchTerminals,
  terminalKeys,
  terminalScope,
  TerminalApiError,
} from "@/systems/terminal";
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
import { WindowManagerRuntime } from "../../../runtime/window-manager-runtime";
import { useTerminalWindowCreation } from "../hooks/use-terminal-window-creation";
import { TerminalAggregateLocation } from "../terminal-aggregate-location";

vi.mock("@/systems/terminal/adapters/terminal-api", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/terminal/adapters/terminal-api")>();
  return { ...actual, fetchTerminals: vi.fn(), closeTerminal: vi.fn(), createTerminal: vi.fn() };
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

beforeEach(() => {
  vi.mocked(createTerminal).mockReset();
  vi.mocked(executeWindowManagerCommand).mockReset();
  vi.mocked(fetchTerminals).mockReset().mockResolvedValue([DEV_SERVER_TERMINAL]);
  vi.mocked(closeTerminal).mockReset().mockResolvedValue(terminalExit);
  vi.spyOn(toast, "error")
    .mockClear()
    .mockImplementation(() => "toast");
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
});

describe("terminal window close lifecycle", () => {
  function setup() {
    const controller = new TerminalWindowClose();
    const guard = controller.guard("ws-close", new QueryClient());
    return { controller, guard };
  }

  it.each([null, "ws-desktop"])(
    "Should close a mixed bound group from desktop %s",
    async desktop => {
      const project = { ...DEV_SERVER_TERMINAL, workspace_id: "ws-bound" };
      const user = { ...DEV_SERVER_TERMINAL, id: "user-terminal", workspace_id: "" };
      vi.mocked(fetchTerminals).mockImplementation(async workspace =>
        workspace === "ws-bound" ? [project] : [user]
      );
      const controller = new TerminalWindowClose();
      const client = new QueryClient();
      const affectedKeys = [
        terminalKeys.catalog({ workspaceId: "ws-bound", profileKey: "default" }),
        terminalKeys.catalog({ workspaceId: "@user", profileKey: "default" }),
        terminalKeys.aggregate("", "default"),
        terminalKeys.aggregate("@user,ws-bound", "default"),
      ];
      const unrelated = terminalKeys.catalog({ workspaceId: "ws-other", profileKey: "default" });
      for (const key of [...affectedKeys, unrelated]) client.setQueryData(key, []);
      const guard = controller.guard(desktop, client);
      const bound = (id: string, workspace: string) => ({
        ...terminalWindow,
        instanceKey: id,
        route: { ...terminalWindow.route, search: { workspace } },
      });
      const result = guard([bound(project.id, "ws-bound"), bound(user.id, "@user")], () => true);
      await waitFor(() =>
        expect(controller.confirmation.get()?.terminals).toEqual([project, user])
      );
      controller.confirmation.get()?.answer(true);
      await expect(result).resolves.toBe(true);
      for (const key of affectedKeys) expect(client.getQueryState(key)?.isInvalidated).toBe(true);
      expect(client.getQueryState(unrelated)?.isInvalidated).toBe(false);
      expect(closeTerminal).toHaveBeenCalledWith(
        "ws-bound",
        project.id,
        { profile: project.profile_name },
        "HUP",
        expect.any(AbortSignal)
      );
      expect(closeTerminal).toHaveBeenCalledWith(
        "@user",
        user.id,
        { profile: user.profile_name },
        "HUP",
        expect.any(AbortSignal)
      );
    }
  );

  it("Should close a missing bound terminal from All projects without confirmation", async () => {
    vi.mocked(fetchTerminals).mockResolvedValue([]);
    const controller = new TerminalWindowClose();
    const guard = controller.guard(null, new QueryClient());
    await expect(
      guard(
        [
          {
            ...terminalWindow,
            route: { ...terminalWindow.route, search: { workspace: "ws-bound" } },
          },
        ],
        () => true
      )
    ).resolves.toBe(true);
    expect(controller.confirmation.get()).toBeNull();
    expect(closeTerminal).not.toHaveBeenCalled();
    expect(toast.error).not.toHaveBeenCalled();
  });

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

// Invariant (W-20): an unbound Terminal window on the All projects desktop lists every project's
// terminals with project badges, and opening one — or a new one in the project the operator
// picks — binds this window to that terminal's own scope in place (route `workspace`).
describe("terminal window under All projects", () => {
  const workspaces = [{ id: "ws-atlas", name: "atlas" }];
  const userTerminal = { ...DEV_SERVER_TERMINAL, id: "term-user", title: "home", workspace_id: "" };

  function renderAggregate(
    overrides: {
      requestedId?: string | null;
      terminals?: (typeof DEV_SERVER_TERMINAL)[];
      projectFilterValues?: string[];
    } = {}
  ) {
    const coordinator = { userRetarget: vi.fn().mockResolvedValue(undefined) };
    render(
      createElement(TerminalAggregateLocation, {
        windowId: "w-terminal",
        catalog: {
          data: overrides.terminals ?? [DEV_SERVER_TERMINAL, userTerminal],
          error: null,
          isPending: false,
          refetch: vi.fn(),
        },
        coordinator,
        projectFilter: {
          field: null,
          values: overrides.projectFilterValues ?? [],
          setValues: vi.fn(),
        },
        requestedId: overrides.requestedId ?? null,
        workspaces,
        profile: "work",
        readOnly: false,
        viewer: { id: "client:web", attachmentToken: "token" },
      })
    );
    return coordinator;
  }

  it("Should name each terminal's project and open it bound to that scope", () => {
    const coordinator = renderAggregate();
    const row = screen.getByTestId(`terminal-aggregate-row-${DEV_SERVER_TERMINAL.id}`);
    expect(row).toHaveTextContent("atlas");
    expect(screen.getByTestId("terminal-aggregate-row-term-user")).toHaveTextContent("User");

    fireEvent.click(screen.getByTestId("terminal-aggregate-row-term-user"));
    expect(coordinator.userRetarget).toHaveBeenCalledWith("w-terminal", {
      app: "terminal",
      instanceKey: "term-user",
      route: { pathname: "/terminal/term-user", search: { workspace: "@user" } },
    });
  });

  it("Should report a narrowed empty list as no match, not as no terminals", () => {
    renderAggregate({ terminals: [], projectFilterValues: ["ws-atlas"] });
    expect(screen.getByTestId("terminal-aggregate-empty")).toHaveTextContent(
      "No matching terminals"
    );
  });

  it("Should bind a terminal opened by id once the aggregate names its project", async () => {
    const coordinator = renderAggregate({ requestedId: DEV_SERVER_TERMINAL.id });
    await waitFor(() =>
      expect(coordinator.userRetarget).toHaveBeenCalledWith(
        "w-terminal",
        expect.objectContaining({
          instanceKey: DEV_SERVER_TERMINAL.id,
          route: expect.objectContaining({ search: { workspace: "ws-atlas" } }),
        })
      )
    );
  });

  it("Should open a new terminal in the chosen project and bind to it", async () => {
    vi.mocked(createTerminal).mockResolvedValue({ ...DEV_SERVER_TERMINAL, id: "term-new" });
    const coordinator = renderAggregate();
    fireEvent.click(screen.getByRole("button", { name: "Open" }));

    await waitFor(() =>
      expect(createTerminal).toHaveBeenCalledWith(
        "ws-atlas",
        {},
        { profile: "work" },
        { id: "client:web", attachmentToken: "token" }
      )
    );
    await waitFor(() =>
      expect(coordinator.userRetarget).toHaveBeenCalledWith(
        "w-terminal",
        expect.objectContaining({
          instanceKey: "term-new",
          route: { pathname: "/terminal/term-new", search: { workspace: "ws-atlas" } },
        })
      )
    );
  });
});
