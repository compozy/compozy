import { act, render, renderHook, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { OverlayContainerContext } from "@compozy/ui";
import {
  WindowNavigationGuardContext,
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
          next: { pathname: string; search: Record<string, unknown> };
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
    mockUseBlocker.mockReturnValue({
      status: "blocked",
      proceed: mockProceed,
      reset: mockReset,
    });

    const { result } = renderHook(() => useUnsavedGuard({ dirty: true, entityName: "coder" }));
    render(<>{result.current.confirmDialog}</>);

    expect(screen.getByTestId("unsaved-guard-dialog")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Discard unsaved changes?" })).toBeInTheDocument();
    expect(screen.getByTestId("unsaved-guard-discard")).toBeInTheDocument();
    expect(screen.getByTestId("unsaved-guard-keep-editing")).toBeInTheDocument();

    await user.click(screen.getByTestId("unsaved-guard-discard"));
    expect(mockProceed).toHaveBeenCalledTimes(1);

    await user.click(screen.getByTestId("unsaved-guard-keep-editing"));
    expect(mockReset).toHaveBeenCalledTimes(1);
  });

  describe("inside a window of the desktop shell [B-1]", () => {
    function shell(port: Partial<WindowNavigationGuardPort>) {
      const windows: WindowNavigationGuardPort = {
        replacesWindowContent: vi.fn(() => false),
        isWindowShown: vi.fn(() => true),
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
      return { windows, pane, wrapper };
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
