import { act, render, renderHook, screen } from "@testing-library/react";
import { createAtom } from "@xstate/store";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { OverlayContainerContext } from "@compozy/ui";
import {
  WindowNavigationGuardContext,
  type UnsavedCloseQuestion,
  type UnsavedWindowEdits,
  type WindowNavigationGuardPort,
} from "@/hooks/use-window-navigation-guard";
import { WindowScopeContext } from "@/hooks/use-window-scope";

const { mockUseBlocker, mockProceed, mockReset } = vi.hoisted(() => {
  const mockProceed = vi.fn();
  const mockReset = vi.fn();
  return {
    mockProceed,
    mockReset,
    mockUseBlocker: vi.fn(
      (_options: {
        shouldBlockFn: (args?: {
          next: { pathname: string; search: Record<string, unknown>; routeId?: string };
        }) => boolean;
        disabled?: boolean;
        enableBeforeUnload?: boolean;
        withResolver?: boolean;
      }) => ({
        status: "idle" as "idle" | "blocked",
        proceed: mockProceed,
        reset: mockReset,
      })
    ),
  };
});

vi.mock("@tanstack/react-router", () => ({
  useBlocker: mockUseBlocker,
}));

import { useUnsavedGuard } from "../use-unsaved-guard";

describe("useUnsavedGuard", () => {
  beforeEach(() => {
    mockProceed.mockReset();
    mockReset.mockReset();
    mockUseBlocker.mockClear();
    mockUseBlocker.mockReturnValue({
      status: "idle",
      proceed: mockProceed,
      reset: mockReset,
    });
  });

  it("Should disable the blocker when pristine", () => {
    renderHook(() => useUnsavedGuard({ dirty: false, entityName: "coder" }));

    expect(mockUseBlocker).toHaveBeenCalledWith(
      expect.objectContaining({
        disabled: true,
        enableBeforeUnload: false,
        withResolver: true,
      })
    );
  });

  it("Should enable blocking when dirty and expose proceed/reset while blocked", () => {
    mockUseBlocker.mockReturnValue({
      status: "blocked",
      proceed: mockProceed,
      reset: mockReset,
    });

    const { result } = renderHook(() => useUnsavedGuard({ dirty: true, entityName: "coder" }));

    expect(mockUseBlocker).toHaveBeenCalledWith(
      expect.objectContaining({
        disabled: false,
        enableBeforeUnload: true,
        withResolver: true,
      })
    );
    expect(result.current.status).toBe("blocked");
    expect(result.current.proceed).toBeTypeOf("function");
    expect(result.current.reset).toBeTypeOf("function");

    act(() => {
      result.current.proceed?.();
      result.current.reset?.();
    });

    expect(mockProceed).toHaveBeenCalledTimes(1);
    expect(mockReset).toHaveBeenCalledTimes(1);
  });

  it("Should keep proceed/reset undefined when idle", () => {
    const { result } = renderHook(() => useUnsavedGuard({ dirty: true, entityName: "coder" }));

    expect(result.current.status).toBe("idle");
    expect(result.current.proceed).toBeUndefined();
    expect(result.current.reset).toBeUndefined();
  });

  it("Should keep the blocker predicate stable while the dirty state is unchanged", () => {
    const { rerender } = renderHook(
      ({ dirty }) => useUnsavedGuard({ dirty, entityName: "coder" }),
      { initialProps: { dirty: true } }
    );
    const firstPredicate = mockUseBlocker.mock.calls.at(-1)?.[0].shouldBlockFn;

    rerender({ dirty: true });

    const secondPredicate = mockUseBlocker.mock.calls.at(-1)?.[0].shouldBlockFn;
    expect(secondPredicate).toBe(firstPredicate);
    expect(secondPredicate?.()).toBe(true);
  });

  it("Should mount the confirmation dialog and wire confirm/cancel to proceed/reset", async () => {
    const user = userEvent.setup();
    const onDiscard = vi.fn();
    mockUseBlocker.mockReturnValue({
      status: "blocked",
      proceed: mockProceed,
      reset: mockReset,
    });

    const { result, unmount } = renderHook(() =>
      useUnsavedGuard({ dirty: true, entityName: "coder", onDiscard })
    );
    render(<>{result.current.confirmDialog}</>);

    expect(screen.getByTestId("unsaved-guard-dialog")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Discard unsaved changes?" })).toBeInTheDocument();
    expect(screen.getByTestId("unsaved-guard-discard")).toBeInTheDocument();
    expect(screen.getByTestId("unsaved-guard-keep-editing")).toBeInTheDocument();

    await user.click(screen.getByTestId("unsaved-guard-discard"));
    expect(mockProceed).toHaveBeenCalledTimes(1);
    // Discard drops the edits once the editor goes (#1653): another guard may still hold the
    // navigation, and then the edits stay.
    expect(onDiscard).not.toHaveBeenCalled();

    await user.click(screen.getByTestId("unsaved-guard-keep-editing"));
    expect(mockReset).toHaveBeenCalledTimes(1);
    unmount();
    expect(onDiscard).toHaveBeenCalledTimes(1);
  });

  it("Should drop only the left resource's edits when a reused editor moves on [#1653]", () => {
    mockUseBlocker.mockReturnValue({ status: "blocked", proceed: mockProceed, reset: mockReset });
    const discardA = vi.fn();
    const discardB = vi.fn();
    const { result, rerender, unmount } = renderHook(
      ({ resourceKey, onDiscard }: { resourceKey: string; onDiscard: () => void }) =>
        useUnsavedGuard({ dirty: true, entityName: "Permissions", onDiscard, resourceKey }),
      { initialProps: { resourceKey: "agent-a", onDiscard: discardA } }
    );

    act(() => result.current.proceed?.());
    expect(discardA).not.toHaveBeenCalled();
    // The editor stays mounted and resumes another agent's dirty draft.
    rerender({ resourceKey: "agent-b", onDiscard: discardB });
    expect(discardA).toHaveBeenCalledTimes(1);

    unmount();
    expect(discardB).not.toHaveBeenCalled();
    expect(discardA).toHaveBeenCalledTimes(1);
  });

  describe("inside a window of the desktop shell [B-1]", () => {
    function shell(port: Partial<WindowNavigationGuardPort>) {
      const held: Array<{ windowId: string; edits: UnsavedWindowEdits }> = [];
      const question = createAtom<UnsavedCloseQuestion | null>(null);
      const windows: WindowNavigationGuardPort = {
        replacesWindowContent: vi.fn(() => false),
        isWindowShown: vi.fn(() => true),
        unsavedEdits: {
          question,
          hold: (windowId, edits) => {
            const entry = { windowId, edits };
            held.push(entry);
            return () => void held.splice(held.indexOf(entry), 1);
          },
        },
        ...port,
      };
      const pane = document.createElement("div");
      document.body.append(pane);
      const wrapper = ({ children }: { children: ReactNode }) => (
        <WindowNavigationGuardContext value={windows}>
          <WindowScopeContext value="w-editor">
            <OverlayContainerContext value={pane}>{children}</OverlayContainerContext>
          </WindowScopeContext>
        </WindowNavigationGuardContext>
      );
      return { windows, pane, wrapper, held, question };
    }

    it("Should ask only when the shell lands the navigation over the editor in its own window", () => {
      const replaces = vi.fn((_windowId: string, next: { pathname: string }) =>
        next.pathname.startsWith("/agents")
      );
      const { wrapper, pane } = shell({ replacesWindowContent: replaces });
      renderHook(() => useUnsavedGuard({ dirty: true, entityName: "coder" }), { wrapper });
      const shouldBlock = mockUseBlocker.mock.calls.at(-1)?.[0].shouldBlockFn;

      // Another window's route (a tab, the dock, Back to it, its own in-window navigation).
      expect(shouldBlock?.({ next: { pathname: "/settings/general", search: {} } })).toBe(false);
      // The editor's window would show another route: the edits would go.
      expect(shouldBlock?.({ next: { pathname: "/agents", search: { tab: "list" } } })).toBe(true);
      expect(replaces).toHaveBeenLastCalledWith("w-editor", {
        pathname: "/agents",
        search: { tab: "list" },
      });
      // The shell answers for routes outside it, so it is told which route matched.
      shouldBlock?.({
        next: { pathname: "/design-system", search: {}, routeId: "/design-system" },
      });
      expect(replaces).toHaveBeenLastCalledWith("w-editor", {
        pathname: "/design-system",
        search: {},
        routeId: "/design-system",
      });
      pane.remove();
    });

    // Invariant (#1653): a dirty editor holds its window, so an operator close of it asks the
    //   guard's question; Keep editing cancels the close, Discard drops the edits with it.
    it("Should hold its window only while dirty", () => {
      const { wrapper, pane, held } = shell({});
      const { rerender } = renderHook(
        ({ dirty }) => useUnsavedGuard({ dirty, entityName: "Memory" }),
        { wrapper, initialProps: { dirty: false } }
      );
      expect(held).toEqual([]);
      rerender({ dirty: true });
      expect(held.map(entry => [entry.windowId, entry.edits.entityName])).toEqual([
        ["w-editor", "Memory"],
      ]);
      rerender({ dirty: false });
      expect(held).toEqual([]);
      pane.remove();
    });

    it.each([
      [1, "Your edits to Memory will be lost."],
      [2, "Your edits in 2 windows will be lost."],
    ])(
      "Should ask an operator close covering %i window(s) and answer it",
      async (windowCount, description) => {
        const user = userEvent.setup();
        const onDiscard = vi.fn();
        const { wrapper, pane, held, question } = shell({});
        function Editor() {
          return (
            <>{useUnsavedGuard({ dirty: true, entityName: "Memory", onDiscard }).confirmDialog}</>
          );
        }
        render(<Editor />, { wrapper });
        const answer = vi.fn((_discard: boolean) => question.set(null));
        const ask = () =>
          act(() => question.set({ presenter: held[0]!.edits, windowCount, answer }));

        ask();
        expect(screen.getByText(description)).toBeInTheDocument();
        expect(pane.contains(screen.getByTestId("unsaved-guard-dialog"))).toBe(true);
        await user.click(screen.getByTestId("unsaved-guard-keep-editing"));
        expect(answer).toHaveBeenLastCalledWith(false);
        expect(screen.queryByTestId("unsaved-guard-dialog")).not.toBeInTheDocument();

        ask();
        await user.click(screen.getByTestId("unsaved-guard-discard"));
        expect(answer).toHaveBeenLastCalledWith(true);
        // The shell discards the edits it closes with the window.
        held[0]!.edits.discard();
        expect(onDiscard).toHaveBeenCalledTimes(1);
        expect(mockProceed).not.toHaveBeenCalled();
        pane.remove();
      }
    );

    it("Should leave a close question another editor presents to it", () => {
      const { wrapper, pane, question } = shell({});
      function Editor() {
        return <>{useUnsavedGuard({ dirty: true, entityName: "Memory" }).confirmDialog}</>;
      }
      render(<Editor />, { wrapper });
      act(() =>
        question.set({
          presenter: { entityName: "Heartbeat", discard: vi.fn() },
          windowCount: 2,
          answer: vi.fn(),
        })
      );
      expect(screen.queryByTestId("unsaved-guard-dialog")).not.toBeInTheDocument();
      pane.remove();
    });

    it.each([
      ["in its window while the window is on screen", true],
      ["on the shell while the window is hidden", false],
    ])("Should show the question %s", (_label, shown) => {
      mockUseBlocker.mockReturnValue({ status: "blocked", proceed: mockProceed, reset: mockReset });
      const { wrapper, pane } = shell({ isWindowShown: () => shown });
      function Editor() {
        return <>{useUnsavedGuard({ dirty: true, entityName: "coder" }).confirmDialog}</>;
      }
      render(<Editor />, { wrapper });

      const dialog = screen.getByTestId("unsaved-guard-dialog");
      expect(pane.contains(dialog)).toBe(shown);
      pane.remove();
    });
  });
});
