// Suite: the shell's focused-window session controls
// Invariant: the sidebar shortcut toggles the sidebar of the focused session window, including a
// window that never chose a worktree; the shell entry the body resolves worktrees from stays.
// Owning layer: FocusedSessionControlsHost, which the shell body's ⌘B and session cycling read.
// No prior suite owned the shell's side of the toggle (the rail's store owns it per scope).
// Boundary IN: the host, the real useSessionSidebarState and its persisted store.
// Boundary OUT: the session list query (useSessionListView) and the Sessions dialog.
import { act, render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { WindowScopeContext } from "@/hooks/use-window-scope";
import {
  resolveSessionSidebarScope,
  sessionSidebarStore,
} from "@/systems/session/hooks/use-session-sidebar-state";
import { SHELL_WORKTREE_SCOPE } from "@/systems/workspace";

import {
  FocusedSessionControlsHost,
  type FocusedSessionControls,
} from "../focused-session-controls";

vi.mock("@/systems/session", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/session")>()),
  useSessionListView: vi.fn(() => ({ search: "", catalog: null })),
}));
vi.mock("../desktop-shell-dialogs", () => ({ DesktopSessionDialogs: () => null }));

const SESSION_WINDOW = "window:session";

describe("FocusedSessionControlsHost", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("Should toggle the focused session window's rail when it never chose a worktree", () => {
    const controlsRef: { current: FocusedSessionControls | null } = { current: null };
    // The shell body sits under the shell entry this window's worktree resolves from; the host
    // sits under the focused window itself, as `FocusedWindowScope` provides it.
    render(
      <WindowScopeContext value={SHELL_WORKTREE_SCOPE}>
        <WindowScopeContext value={SESSION_WINDOW}>
          <FocusedSessionControlsHost
            controlsRef={controlsRef}
            dataScope={null}
            worktreeId={undefined}
            worktreeResolved
            workspaceId={null}
            open={false}
            onOpenChange={() => {}}
            attentionDisconnected={false}
            onNewSession={() => {}}
          />
        </WindowScopeContext>
      </WindowScopeContext>
    );
    const shellOpen = resolveSessionSidebarScope(
      sessionSidebarStore.getSnapshot().context.byScope,
      SHELL_WORKTREE_SCOPE
    ).open;
    const windowOpen = controlsRef.current?.sidebar.open;
    expect(windowOpen).toBeTypeOf("boolean");

    act(() => controlsRef.current?.sidebar.toggle());

    const byScope = sessionSidebarStore.getSnapshot().context.byScope;
    // The rail the session pane reads (its own window scope) flips; the shell entry does not.
    expect(resolveSessionSidebarScope(byScope, SESSION_WINDOW).open).toBe(!windowOpen);
    expect(resolveSessionSidebarScope(byScope, SHELL_WORKTREE_SCOPE).open).toBe(shellOpen);
    expect(controlsRef.current?.sidebar.open).toBe(!windowOpen);
  });
});
