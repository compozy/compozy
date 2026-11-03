import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { PropsWithChildren } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { primaryAgentFixture } from "@/systems/agent/testing";
import { resetProfileViews, setProfileView } from "@/systems/profiles";

const mocks = vi.hoisted(() => ({
  agent: undefined as unknown,
  navigate: vi.fn(),
  updateMutate: vi.fn(),
  updateMutateAsync: vi.fn(),
  refetch: vi.fn(),
  toastSuccess: vi.fn(),
  workspaceId: "ws-test",
  settingsProviders: { isLoading: false, isFetching: false },
}));

vi.mock("@tanstack/react-router", () => ({ useNavigate: () => mocks.navigate }));
vi.mock("sonner", () => ({ toast: { success: mocks.toastSuccess } }));
vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspaceId: mocks.workspaceId,
    activeWorkspace: { name: "Test" },
    runtimeWorkspaceId: mocks.workspaceId,
    registeredWorkspaces: mocks.workspaceId ? [{ id: mocks.workspaceId, name: "Test" }] : [],
  }),
  useDataScope: () =>
    mocks.workspaceId ? { kind: "project", workspaceId: mocks.workspaceId } : { kind: "all" },
}));
vi.mock("@/systems/workspace/hooks/use-workspaces", () => ({
  useWorkspace: () => ({ data: { providers: [] }, isLoading: false }),
}));
vi.mock("@/systems/settings/hooks/use-settings-collections", () => ({
  useSettingsProviders: () => ({ data: { providers: [] }, ...mocks.settingsProviders }),
}));
vi.mock("@/systems/model-catalog/hooks/use-runtime-model-catalog", () => ({
  useRuntimeModelCatalog: () => ({
    models: [],
    loading: false,
    loaded: true,
    refreshing: false,
    error: null,
    refresh: vi.fn(),
  }),
}));
vi.mock("@/systems/model-catalog/lib/to-runtime-selector-options", () => ({
  providerNeedsAuth: () => false,
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
import { useAgentRuntimeEditor } from "../use-agent-runtime-editor";
import { useAgentSettingsPage } from "../use-agent-settings-page";

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: PropsWithChildren) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useAgentSettingsPage", () => {
  afterEach(() => act(() => resetProfileViews()));
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.updateMutateAsync = vi.fn();
    mocks.refetch = vi.fn();
    mocks.agent = { ...primaryAgentFixture, origin: "workspace" };
    mocks.workspaceId = "ws-test";
    mocks.settingsProviders = { isLoading: false, isFetching: false };
    resetProfileViews();
    mocks.refetch.mockResolvedValue({
      data: { ...primaryAgentFixture, origin: "workspace", prompt: "Reloaded prompt" },
    });
  });

  it("Should save one whole-definition draft with CAS and return to pristine", async () => {
    setProfileView({ kind: "profile", profile: "open-design" });
    const { result } = renderHook(
      () => useAgentSettingsPage({ name: primaryAgentFixture.name, section: "instructions" }),
      { wrapper: createWrapper() }
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
      profile: "open-design",
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
    const { result, rerender } = renderHook(
      () => useAgentSettingsPage({ name: primaryAgentFixture.name, section: "instructions" }),
      { wrapper: createWrapper() }
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
    const { result, rerender } = renderHook(
      () => useAgentSettingsPage({ name: primaryAgentFixture.name, section: "instructions" }),
      { wrapper: createWrapper() }
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
    const { result } = renderHook(
      () => useAgentSettingsPage({ name: primaryAgentFixture.name, section: "instructions" }),
      { wrapper: createWrapper() }
    );
    await waitFor(() => expect(result.current.draft).not.toBeNull());
    act(() => result.current.patchDraft({ prompt: "Denied prompt" }));
    mocks.updateMutateAsync.mockRejectedValueOnce(new AgentApiError("forbidden", 403));
    act(() => result.current.onSave());

    await waitFor(() => expect(result.current.phase).toBe("denied"));
    expect(result.current.saveBlocked).toBe(true);
    expect(result.current.saveBlockedCaption).toBe("Editing is not permitted for this agent.");
  });

  it("Should report global providers as loading only until they first arrive, not while they refetch", () => {
    mocks.agent = { ...primaryAgentFixture, origin: "global" };
    mocks.settingsProviders = { isLoading: false, isFetching: true };
    const { result, rerender } = renderHook(
      () => useAgentSettingsPage({ name: primaryAgentFixture.name, section: "runtime" }),
      { wrapper: createWrapper() }
    );

    // A background refetch keeps the provider control as it is.
    expect(result.current.providersLoading).toBe(false);

    mocks.settingsProviders = { isLoading: true, isFetching: true };
    rerender();
    expect(result.current.providersLoading).toBe(true);
  });

  it("Should navigate between sections, detail, and provider settings", async () => {
    const { result } = renderHook(
      () => useAgentSettingsPage({ name: primaryAgentFixture.name, section: "basics" }),
      { wrapper: createWrapper() }
    );
    await waitFor(() => expect(result.current.draft).not.toBeNull());
    act(() => {
      result.current.setSection("mcp");
      result.current.onBackToDetail();
      result.current.onOpenProviderSettings();
    });
    expect(mocks.navigate).toHaveBeenCalledWith({
      to: "/agents/$name/settings",
      params: { name: primaryAgentFixture.name },
      search: { section: "mcp" },
      replace: true,
    });
    expect(mocks.navigate).toHaveBeenCalledWith({
      to: "/agents/$name",
      params: { name: primaryAgentFixture.name },
      search: {},
    });
    expect(mocks.navigate).toHaveBeenCalledWith({ to: "/settings/providers" });
  });

  it.each(["agent", "profile"] as const)(
    "Should replace the draft on %s changes without leaking dirty state",
    dimension => {
      const renderedPrompts: Array<string | null> = [];
      const { result, rerender } = renderHook(
        ({ name }: { name: string }) => {
          const page = useAgentSettingsPage({ name, section: "instructions" });
          renderedPrompts.push(page.draft?.prompt ?? null);
          return page;
        },
        { initialProps: { name: primaryAgentFixture.name }, wrapper: createWrapper() }
      );

      act(() => result.current.patchDraft({ prompt: "Dirty prompt" }));
      expect(result.current.draft?.prompt).toBe("Dirty prompt");

      mocks.agent = {
        ...primaryAgentFixture,
        name: dimension === "agent" ? "replacement-agent" : primaryAgentFixture.name,
        definition_digest: "replacement-digest",
        prompt: "Replacement prompt",
        origin: "workspace",
      };
      renderedPrompts.length = 0;
      if (dimension === "profile") {
        act(() => setProfileView({ kind: "profile", profile: "open-design" }));
      }
      rerender({ name: dimension === "agent" ? "replacement-agent" : primaryAgentFixture.name });

      expect(renderedPrompts.length).toBeGreaterThan(0);
      expect(renderedPrompts.every(prompt => prompt === "Replacement prompt")).toBe(true);
      expect(result.current.dirty).toBe(false);
    }
  );
});

// Invariant: the agent home's Model row saves a pick, or a return to the default, in one
//   digest-guarded update that keeps the rest of the definition.
describe("useAgentRuntimeEditor", () => {
  afterEach(() => act(() => resetProfileViews()));
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.workspaceId = "ws-test";
    resetProfileViews();
  });

  function renderEditor(agent: unknown) {
    mocks.agent = agent;
    return renderHook(() => useAgentRuntimeEditor({ agent: mocks.agent as never }), {
      wrapper: createWrapper(),
    });
  }

  it("Should save a picked model at once and keep the agent's command", () => {
    const { result } = renderEditor({
      ...primaryAgentFixture,
      origin: "workspace",
      provider: "",
      model: "",
      reasoning_effort: "",
      command: "my-claude --fast",
    });
    expect(result.current.overridden).toBe(false);

    act(() =>
      result.current.onChange({ provider: "claude", model: "sonnet", reasoning_effort: "high" })
    );

    expect(mocks.updateMutate).toHaveBeenCalledOnce();
    expect(mocks.updateMutate.mock.calls[0]![0]).toMatchObject({
      name: primaryAgentFixture.name,
      cacheWorkspace: "ws-test",
      params: {
        expected_digest: primaryAgentFixture.definition_digest,
        workspace: "ws-test",
        agent: {
          provider: "claude",
          model: "sonnet",
          reasoning_effort: "high",
          command: "my-claude --fast",
        },
      },
    });
  });

  it("Should apply a choice made during a save on the saved definition, not drop it", () => {
    const agent = { ...primaryAgentFixture, origin: "workspace", provider: "", model: "" };
    const { result } = renderEditor(agent);

    act(() =>
      result.current.onChange({ provider: "claude", model: "sonnet", reasoning_effort: "" })
    );
    act(() => result.current.onChange({ provider: "claude", model: "opus", reasoning_effort: "" }));
    expect(mocks.updateMutate).toHaveBeenCalledOnce();

    const saved = { ...agent, provider: "claude", model: "sonnet", definition_digest: "saved" };
    act(() => mocks.updateMutate.mock.calls[0]![1].onSuccess(saved));

    expect(mocks.updateMutate).toHaveBeenCalledTimes(2);
    expect(mocks.updateMutate.mock.calls[1]![0].params).toMatchObject({
      expected_digest: "saved",
      agent: { provider: "claude", model: "opus" },
    });
  });

  it("Should drop the agent's own runtime when it goes back to the default", () => {
    const { result } = renderEditor({
      ...primaryAgentFixture,
      origin: "workspace",
      provider: "claude",
      model: "sonnet",
      reasoning_effort: "high",
      speed: "fast",
    });
    expect(result.current.overridden).toBe(true);

    act(() => result.current.onUseDefault());

    const { agent } = mocks.updateMutate.mock.calls[0]![0].params;
    expect(agent).not.toHaveProperty("provider");
    expect(agent).not.toHaveProperty("model");
    expect(agent).not.toHaveProperty("reasoning_effort");
    expect(agent).not.toHaveProperty("speed");
    expect(agent.acp_options).toEqual([]);
  });
});
