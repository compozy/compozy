import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { primaryAgentFixture } from "@/systems/agent/testing";

const mocks = vi.hoisted(() => ({
  agent: undefined as unknown,
  navigate: vi.fn(),
  updateMutate: vi.fn(),
  updateMutateAsync: vi.fn(),
  refetch: vi.fn(),
  toastSuccess: vi.fn(),
  workspaceId: "ws-test",
}));

vi.mock("@tanstack/react-router", () => ({ useNavigate: () => mocks.navigate }));
vi.mock("sonner", () => ({ toast: { success: mocks.toastSuccess } }));
vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspaceId: mocks.workspaceId,
    activeWorkspace: { name: "Test" },
  }),
  useWorkspace: () => ({ data: { providers: [] }, isLoading: false }),
}));
vi.mock("@/systems/settings", () => ({
  useSettingsProviders: () => ({ data: { providers: [] }, isLoading: false, isFetching: false }),
}));
vi.mock("@/systems/model-catalog", () => ({
  providerNeedsAuth: () => false,
  useRuntimeModelCatalog: () => ({
    models: [],
    loading: false,
    loaded: true,
    refreshing: false,
    error: null,
    refresh: vi.fn(),
  }),
}));
vi.mock("../use-agents", () => ({
  useAgent: () => ({
    data: mocks.agent,
    isLoading: false,
    error: null,
    refetch: mocks.refetch,
  }),
  useUpdateAgent: () => ({
    mutate: mocks.updateMutate,
    mutateAsync: mocks.updateMutateAsync,
    isPending: false,
  }),
}));
vi.mock("../use-agent-delete-flow", () => ({
  useAgentDeleteFlow: () => ({
    open: false,
    openDialog: vi.fn(),
    closeDialog: vi.fn(),
    confirmDialog: null,
    isDeleting: false,
  }),
}));
vi.mock("../use-unsaved-guard", () => ({
  useUnsavedGuard: () => ({ confirmDialog: null }),
}));

import { AgentApiError, AgentDigestConflictError } from "../../adapters/agent-api";
import { useAgentSettingsPage } from "../use-agent-settings-page";

describe("useAgentSettingsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.updateMutateAsync = vi.fn();
    mocks.refetch = vi.fn();
    mocks.agent = { ...primaryAgentFixture, origin: "workspace" };
    mocks.workspaceId = "ws-test";
    mocks.refetch.mockResolvedValue({
      data: { ...primaryAgentFixture, origin: "workspace", prompt: "Reloaded prompt" },
    });
  });

  it("Should save one whole-definition draft with CAS and return to pristine", async () => {
    const { result } = renderHook(() =>
      useAgentSettingsPage({ name: primaryAgentFixture.name, section: "instructions" })
    );
    await waitFor(() => expect(result.current.draft).not.toBeNull());

    act(() => result.current.patchDraft({ prompt: "Updated prompt" }));
    expect(result.current.dirty).toBe(true);
    mocks.updateMutateAsync.mockResolvedValueOnce({
      ...primaryAgentFixture,
      origin: "workspace",
      prompt: "Updated prompt",
    });
    act(() => {
      result.current.onSave();
      result.current.onSave();
    });
    expect(mocks.updateMutateAsync).toHaveBeenCalledOnce();

    const variables = mocks.updateMutateAsync.mock.calls.at(-1)![0];
    expect(variables).toMatchObject({
      name: primaryAgentFixture.name,
      params: {
        expected_digest: primaryAgentFixture.definition_digest,
        workspace: "ws-test",
        agent: { prompt: "Updated prompt" },
      },
    });

    await waitFor(() => expect(result.current.dirty).toBe(false));
    expect(mocks.toastSuccess).toHaveBeenCalledWith("Changes saved");
    expect(result.current.dirty).toBe(false);
  });

  it("Should render conflict recovery and reload the fresh digest before retry", async () => {
    const { result, rerender } = renderHook(() =>
      useAgentSettingsPage({ name: primaryAgentFixture.name, section: "instructions" })
    );
    await waitFor(() => expect(result.current.draft).not.toBeNull());
    act(() => result.current.patchDraft({ prompt: "Conflicting prompt" }));
    mocks.updateMutateAsync.mockRejectedValueOnce(new AgentDigestConflictError("stale"));
    act(() => result.current.onSave());

    await waitFor(() => expect(result.current.phase).toBe("conflict"));
    expect(result.current.error).toMatch(/changed elsewhere/i);
    const previousRefetch = mocks.refetch;
    const currentRefetch = vi.fn().mockResolvedValue({
      data: { ...primaryAgentFixture, origin: "workspace", prompt: "Reloaded prompt" },
    });
    mocks.refetch = currentRefetch;
    rerender();
    act(() => result.current.onReloadAndRetry());
    await waitFor(() => expect(currentRefetch).toHaveBeenCalledTimes(1));
    expect(previousRefetch).not.toHaveBeenCalled();
    await waitFor(() => expect(result.current.draft?.prompt).toBe("Reloaded prompt"));
  });

  it("Should save through the mutation callback from the render that issues the intent", async () => {
    const { result, rerender } = renderHook(() =>
      useAgentSettingsPage({ name: primaryAgentFixture.name, section: "instructions" })
    );
    await waitFor(() => expect(result.current.draft).not.toBeNull());
    act(() => result.current.patchDraft({ prompt: "Current callback prompt" }));
    const previousSave = mocks.updateMutateAsync;
    const currentSave = vi.fn().mockResolvedValue({
      ...primaryAgentFixture,
      origin: "workspace",
      prompt: "Current callback prompt",
    });
    mocks.updateMutateAsync = currentSave;
    rerender();

    act(() => result.current.onSave());

    await waitFor(() => expect(result.current.dirty).toBe(false));
    expect(previousSave).not.toHaveBeenCalled();
    expect(currentSave).toHaveBeenCalledOnce();
  });

  it("Should keep Save focusable-but-blocked with a caption after a permission denial", async () => {
    const { result } = renderHook(() =>
      useAgentSettingsPage({ name: primaryAgentFixture.name, section: "instructions" })
    );
    await waitFor(() => expect(result.current.draft).not.toBeNull());
    act(() => result.current.patchDraft({ prompt: "Denied prompt" }));
    mocks.updateMutateAsync.mockRejectedValueOnce(new AgentApiError("forbidden", 403));
    act(() => result.current.onSave());

    await waitFor(() => expect(result.current.phase).toBe("denied"));
    expect(result.current.saveBlocked).toBe(true);
    expect(result.current.saveBlockedCaption).toBe("Editing is not permitted for this agent.");
  });

  it("Should navigate between sections, detail, and provider settings", async () => {
    const { result } = renderHook(() =>
      useAgentSettingsPage({ name: primaryAgentFixture.name, section: "basics" })
    );
    await waitFor(() => expect(result.current.draft).not.toBeNull());
    act(() => {
      result.current.setSection("danger");
      result.current.onBackToDetail();
      result.current.onOpenProviderSettings();
    });
    expect(mocks.navigate).toHaveBeenCalledWith({
      to: "/agents/$name/settings",
      params: { name: primaryAgentFixture.name },
      search: { section: "danger" },
      replace: true,
    });
    expect(mocks.navigate).toHaveBeenCalledWith({
      to: "/agents/$name",
      params: { name: primaryAgentFixture.name },
    });
    expect(mocks.navigate).toHaveBeenCalledWith({ to: "/settings/providers" });
  });

  it("Should expose a replacement agent draft in every render without leaking dirty state", () => {
    const renderedPrompts: Array<string | null> = [];
    const { result, rerender } = renderHook(
      ({ name }: { name: string }) => {
        const page = useAgentSettingsPage({ name, section: "instructions" });
        renderedPrompts.push(page.draft?.prompt ?? null);
        return page;
      },
      { initialProps: { name: primaryAgentFixture.name } }
    );

    act(() => result.current.patchDraft({ prompt: "Dirty prompt" }));
    expect(result.current.draft?.prompt).toBe("Dirty prompt");

    mocks.agent = {
      ...primaryAgentFixture,
      name: "replacement-agent",
      definition_digest: "replacement-digest",
      prompt: "Replacement prompt",
      origin: "workspace",
    };
    renderedPrompts.length = 0;
    rerender({ name: "replacement-agent" });

    expect(renderedPrompts.length).toBeGreaterThan(0);
    expect(renderedPrompts.every(prompt => prompt === "Replacement prompt")).toBe(true);
    expect(result.current.dirty).toBe(false);
  });
});
