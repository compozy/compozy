// Suite: session-create workspace binding
// Invariant: opening a session uses the daemon runtime workspace, including the hidden home row
// while Global scope is active, and seeds the environment from a resolved worktree selection.
// Owning layer: the session-create hook that translates shell scope into store commands.
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SessionCreateProvider } from "../../contexts/session-create-context";
import { createSessionCreateStore } from "../../stores/session-create-store";
import { sessionStore } from "../../stores/session-store";
import { useSessionCreateActions, useSessionCreateHasActiveWorkspace } from "../use-session-create";
import { useSessionPromptFallback } from "../use-session-prompt-fallback";

const workspace = vi.hoisted(() => ({
  runtimeWorkspaceId: "ws_home" as string | null,
  runtimeWorkspace: { default_agent: "general" } as { default_agent?: string } | undefined,
  scope: "global" as "global" | "workspace",
}));
const agents = vi.hoisted(() => ({ data: [{ name: "general" }] as Array<{ name: string }> }));
const createSessionAsync = vi.hoisted(() => vi.fn());
const notifyUser = vi.hoisted(() => vi.fn());
const scopedWorktree = vi.hoisted(() => ({
  id: undefined as string | undefined,
  resolved: true,
}));
const toastError = vi.hoisted(() => vi.fn());

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: null, ...workspace }),
}));

vi.mock("@/systems/workspace/hooks/use-active-worktree", async importOriginal => ({
  ...(await importOriginal<object>()),
  useScopedWorktreeFilter: () => ({
    worktreeId: scopedWorktree.id,
    resolved: scopedWorktree.resolved,
  }),
}));

vi.mock("sonner", () => ({ toast: { error: toastError } }));
vi.mock("@/systems/agent", () => ({ useAgents: () => agents }));
vi.mock("@/lib/user-feedback", () => ({ notifyUser }));
vi.mock("../use-session-actions", () => ({
  useCreateSession: () => ({ mutateAsync: createSessionAsync }),
}));

describe("session create workspace binding", () => {
  beforeEach(() => {
    workspace.runtimeWorkspaceId = "ws_home";
    workspace.runtimeWorkspace = { default_agent: "general" };
    workspace.scope = "global";
    agents.data = [{ name: "general" }];
    scopedWorktree.id = undefined;
    scopedWorktree.resolved = true;
    toastError.mockReset();
    createSessionAsync.mockReset();
    notifyUser.mockReset();
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

  it("Should seed the environment from the acting scope's ready worktree", () => {
    workspace.runtimeWorkspaceId = "ws_git";
    workspace.scope = "workspace";
    scopedWorktree.id = "wt_payments_retry";
    const store = createSessionCreateStore();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <SessionCreateProvider store={store}>{children}</SessionCreateProvider>
    );
    const actions = renderHook(() => useSessionCreateActions(), { wrapper });

    act(() => actions.result.current.openForAgent("general"));

    expect(store.getSnapshot().context).toMatchObject({
      open: true,
      mode: "advanced",
      draft: {
        agentName: "general",
        workspaceId: "ws_git",
        environment: { kind: "worktree", worktreeId: "wt_payments_retry" },
      },
    });
  });

  it("Should open at the workspace root when the acting scope has no worktree", () => {
    workspace.runtimeWorkspaceId = "ws_git";
    workspace.scope = "workspace";
    const store = createSessionCreateStore();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <SessionCreateProvider store={store}>{children}</SessionCreateProvider>
    );
    const actions = renderHook(() => useSessionCreateActions(), { wrapper });

    act(() => actions.result.current.openForAgent("general"));

    expect(store.getSnapshot().context).toMatchObject({
      open: true,
      mode: "simple",
      draft: { agentName: "general", workspaceId: "ws_git", environment: { kind: "root" } },
    });
  });

  it("Should wait for a stored worktree selection before opening", () => {
    workspace.runtimeWorkspaceId = "ws_git";
    workspace.scope = "workspace";
    scopedWorktree.resolved = false;
    const store = createSessionCreateStore();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <SessionCreateProvider store={store}>{children}</SessionCreateProvider>
    );
    const actions = renderHook(() => useSessionCreateActions(), { wrapper });

    act(() => actions.result.current.openForAgent("general"));

    expect(store.getSnapshot().context.open).toBe(false);
    expect(toastError).toHaveBeenCalledWith("Worktrees are not available yet. Try again.");
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

  it("Should send nothing before selection and queue the query only after session creation [UT-141]", async () => {
    const store = createSessionCreateStore();
    const onCreated = vi.fn();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <SessionCreateProvider store={store}>{children}</SessionCreateProvider>
    );
    createSessionAsync.mockResolvedValue({
      id: "session-fallback-141",
      workspace_id: "ws_home",
      agent_name: "general",
    });
    const fallback = renderHook(
      () => useSessionPromptFallback({ onCreated, onPickerOpened: vi.fn() }),
      { wrapper }
    );

    expect(createSessionAsync).not.toHaveBeenCalled();
    expect(sessionStore.getSnapshot().context.firstPrompts["session-fallback-141"]).toBeUndefined();

    await act(async () => fallback.result.current.run("Investigate flaky tests"));

    expect(createSessionAsync).toHaveBeenCalledWith({
      agent_name: "general",
      workspace: "ws_home",
    });
    expect(sessionStore.getSnapshot().context.firstPrompts["session-fallback-141"]).toEqual({
      text: "Investigate flaky tests",
      claimed: false,
    });
    expect(onCreated).toHaveBeenCalledWith(expect.objectContaining({ id: "session-fallback-141" }));
  });

  it("Should open the agent picker with the query when the default cannot resolve [UT-142]", async () => {
    agents.data = [];
    const store = createSessionCreateStore();
    const onPickerOpened = vi.fn();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <SessionCreateProvider store={store}>{children}</SessionCreateProvider>
    );
    const fallback = renderHook(
      () => useSessionPromptFallback({ onCreated: vi.fn(), onPickerOpened }),
      { wrapper }
    );

    await act(async () => fallback.result.current.run("Find the missing route"));

    expect(createSessionAsync).not.toHaveBeenCalled();
    expect(store.getSnapshot().context).toMatchObject({
      open: true,
      draft: { agentName: "", firstMessage: "Find the missing route", workspaceId: "ws_home" },
    });
    expect(onPickerOpened).toHaveBeenCalledTimes(1);
  });

  it("Should report a spawn failure and allow retrying the preserved query [UT-143]", async () => {
    const store = createSessionCreateStore();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <SessionCreateProvider store={store}>{children}</SessionCreateProvider>
    );
    createSessionAsync
      .mockRejectedValueOnce(new Error("agent executable is unavailable"))
      .mockResolvedValueOnce({
        id: "session-fallback-retry",
        workspace_id: "ws_home",
        agent_name: "general",
      });
    const fallback = renderHook(
      () => useSessionPromptFallback({ onCreated: vi.fn(), onPickerOpened: vi.fn() }),
      { wrapper }
    );

    await act(async () => fallback.result.current.run("Repair the build"));
    expect(notifyUser).toHaveBeenCalledWith({
      message: "Could not ask the agent: agent executable is unavailable",
      tone: "error",
    });

    await act(async () => fallback.result.current.run("Repair the build"));
    expect(createSessionAsync).toHaveBeenNthCalledWith(2, {
      agent_name: "general",
      workspace: "ws_home",
    });
    expect(sessionStore.getSnapshot().context.firstPrompts["session-fallback-retry"]?.text).toBe(
      "Repair the build"
    );
  });

  it("Should guard concurrent Enter and allow a deliberate repeat after completion [UT-144]", async () => {
    let resolveFirst: ((value: unknown) => void) | undefined;
    createSessionAsync
      .mockImplementationOnce(
        () =>
          new Promise(resolve => {
            resolveFirst = resolve;
          })
      )
      .mockResolvedValueOnce({
        id: "session-fallback-second",
        workspace_id: "ws_home",
        agent_name: "general",
      });
    const store = createSessionCreateStore();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <SessionCreateProvider store={store}>{children}</SessionCreateProvider>
    );
    const fallback = renderHook(
      () => useSessionPromptFallback({ onCreated: vi.fn(), onPickerOpened: vi.fn() }),
      { wrapper }
    );

    let first: Promise<void> | undefined;
    act(() => {
      first = fallback.result.current.run("Plan the release");
      void fallback.result.current.run("Plan the release");
    });
    expect(createSessionAsync).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolveFirst?.({
        id: "session-fallback-first",
        workspace_id: "ws_home",
        agent_name: "general",
      });
      await first;
    });
    await waitFor(() => expect(fallback.result.current.pending).toBe(false));

    await act(async () => fallback.result.current.run("Plan the release"));
    expect(createSessionAsync).toHaveBeenCalledTimes(2);
  });
});
