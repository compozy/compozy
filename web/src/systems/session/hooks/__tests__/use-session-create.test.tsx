// Suite: session-create workspace binding
// Invariant: opening a session uses the daemon runtime workspace, including the hidden home row
// while Global scope is active.
// Owning layer: the session-create hook that translates shell scope into store commands.
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SessionCreateProvider } from "../../contexts/session-create-context";
import { createSessionCreateStore } from "../../stores/session-create-store";
import { useSessionCreateActions, useSessionCreateHasActiveWorkspace } from "../use-session-create";

const workspace = vi.hoisted(() => ({ runtimeWorkspaceId: "ws_home" as string | null }));
const toastError = vi.hoisted(() => vi.fn());

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: null, ...workspace }),
}));

vi.mock("sonner", () => ({ toast: { error: toastError } }));

describe("session create workspace binding", () => {
  beforeEach(() => {
    workspace.runtimeWorkspaceId = "ws_home";
    toastError.mockReset();
  });

  it("Should open against the hidden home workspace while Global scope is active", () => {
    const store = createSessionCreateStore();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <SessionCreateProvider store={store}>{children}</SessionCreateProvider>
    );
    const actions = renderHook(() => useSessionCreateActions(), { wrapper });
    const availability = renderHook(() => useSessionCreateHasActiveWorkspace());

    act(() => actions.result.current.openForAgent("general"));

    expect(availability.result.current).toBe(true);
    expect(store.getSnapshot().context).toMatchObject({
      open: true,
      draft: { agentName: "general", workspaceId: "ws_home" },
    });
    expect(toastError).not.toHaveBeenCalled();
  });

  it("Should reject opening only when no runtime workspace exists", () => {
    workspace.runtimeWorkspaceId = null;
    const store = createSessionCreateStore();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <SessionCreateProvider store={store}>{children}</SessionCreateProvider>
    );
    const actions = renderHook(() => useSessionCreateActions(), { wrapper });
    const availability = renderHook(() => useSessionCreateHasActiveWorkspace());

    act(() => actions.result.current.openForAgent("general"));

    expect(availability.result.current).toBe(false);
    expect(store.getSnapshot().context.open).toBe(false);
    expect(toastError).toHaveBeenCalledWith(
      "Select an active workspace before starting a session."
    );
  });
});
