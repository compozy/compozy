// Suite: the shell's focused-window session controls
// Invariant: the sidebar shortcut toggles the sidebar of the focused session window, including a
// window that never chose a worktree; the shell's worktree selection stays unchanged while
// the last-used sidebar preference follows the toggle, as the sidebar store specifies.
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
import {
  activeWorkspaceStore,
  selectWorktreeForScope,
  SHELL_WORKTREE_SCOPE,
} from "@/systems/workspace";

import {
  FocusedSessionControlsHost,
  type FocusedSessionControls,
} from "../focused-session-controls";

// Mocked at its defining module, not the `@/systems/session` barrel: the barrel reaches the os
// barrel and back to this host, so a barrel factory hands the host the real hook.
vi.mock("@/systems/session/hooks/use-session-list-view", () => ({
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
    // The window's own rail and the shell entry start apart, so a toggle written to the wrong
    // scope cannot pass for the window's own.
    sessionSidebarStore.trigger.sidebarVisibilityChanged({ scopeId: SESSION_WINDOW, open: true });
    sessionSidebarStore.trigger.sidebarVisibilityChanged({
      scopeId: SHELL_WORKTREE_SCOPE,
      open: false,
    });
    selectWorktreeForScope(SHELL_WORKTREE_SCOPE, "workspace-one", "worktree-one");
    const worktreeSelections = activeWorkspaceStore.getSnapshot().context.worktreeByScope;
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
    const windowOpen = controlsRef.current?.sidebar.open;
    expect(windowOpen).toBe(true);

    act(() => controlsRef.current?.sidebar.toggle());

    const byScope = sessionSidebarStore.getSnapshot().context.byScope;
    // The window's rail and last-used preference flip; the shell's worktree pick stays.
    expect(byScope[SESSION_WINDOW]?.open).toBe(!windowOpen);
    expect(resolveSessionSidebarScope(byScope, SHELL_WORKTREE_SCOPE).open).toBe(!windowOpen);
    expect(activeWorkspaceStore.getSnapshot().context.worktreeByScope).toEqual(worktreeSelections);
    expect(controlsRef.current?.sidebar.open).toBe(!windowOpen);
  });
});
