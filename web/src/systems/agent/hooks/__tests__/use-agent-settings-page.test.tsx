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
//   digest-guarded update that keeps the rest of the definition; a choice made during a save
//   applies on that agent's saved definition, never on another agent's.
describe("useAgentRuntimeEditor", () => {
  type Saved = { resolve: (agent: unknown) => void; reject: (error: unknown) => void };
  let saves: Saved[];

  afterEach(() => act(() => resetProfileViews()));
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.workspaceId = "ws-test";
    resetProfileViews();
    saves = [];
    mocks.updateMutateAsync = vi.fn(
      () => new Promise((resolve, reject) => saves.push({ resolve, reject }))
    );
  });

  const workspaceAgent = (overrides: Record<string, unknown> = {}) => ({
    ...primaryAgentFixture,
    origin: "workspace",
    provider: "",
    model: "",
    reasoning_effort: "",
    ...overrides,
  });

  function renderEditor(agent: unknown) {
    return renderHook(
      ({ current }: { current: unknown }) => useAgentRuntimeEditor({ agent: current as never }),
      { wrapper: createWrapper(), initialProps: { current: agent } }
    );
  }

  const savedParams = (index: number) => mocks.updateMutateAsync.mock.calls[index]![0];

  it("Should save a picked model at once and keep the agent's command", () => {
    const { result } = renderEditor(workspaceAgent({ command: "my-claude --fast" }));
    expect(result.current.overridden).toBe(false);

    act(() =>
      result.current.onChange({ provider: "claude", model: "sonnet", reasoning_effort: "high" })
    );

    expect(mocks.updateMutateAsync).toHaveBeenCalledOnce();
    expect(savedParams(0)).toMatchObject({
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
    // The pick shows while it saves, so the next control builds on it.
    expect(result.current.value).toMatchObject({ provider: "claude", model: "sonnet" });
  });

  it("Should apply a choice made during a save on the saved definition, not drop it", async () => {
    const agent = workspaceAgent();
    const { result } = renderEditor(agent);

    act(() =>
      result.current.onChange({ provider: "claude", model: "sonnet", reasoning_effort: "" })
    );
    act(() =>
      result.current.onChange({ provider: "claude", model: "opus", reasoning_effort: "high" })
    );
    expect(mocks.updateMutateAsync).toHaveBeenCalledOnce();

    await act(async () =>
      saves[0]!.resolve({ ...agent, provider: "claude", model: "sonnet", definition_digest: "s1" })
    );

    expect(mocks.updateMutateAsync).toHaveBeenCalledTimes(2);
    expect(savedParams(1).params).toMatchObject({
      expected_digest: "s1",
      agent: { provider: "claude", model: "opus", reasoning_effort: "high" },
    });
  });

  it("Should save another agent's choice on that agent while the first save runs", async () => {
    const first = workspaceAgent({ name: "triage" });
    const second = workspaceAgent({ name: "reviewer", definition_digest: "reviewer-digest" });
    const { result, rerender } = renderEditor(first);

    act(() =>
      result.current.onChange({ provider: "claude", model: "sonnet", reasoning_effort: "" })
    );
    rerender({ current: second });
    expect(result.current.value.model).toBe("");
    act(() => result.current.onChange({ provider: "claude", model: "opus", reasoning_effort: "" }));

    expect(mocks.updateMutateAsync).toHaveBeenCalledTimes(2);
    expect(savedParams(1)).toMatchObject({
      name: "reviewer",
      params: { expected_digest: "reviewer-digest", agent: { model: "opus" } },
    });

    // Each agent keeps its own pending choice while its save runs.
    rerender({ current: first });
    expect(result.current.value.model).toBe("sonnet");

    await act(async () => saves[0]!.resolve({ ...first, model: "sonnet" }));
    expect(mocks.updateMutateAsync).toHaveBeenCalledTimes(2);
  });

  it("Should hold picks until a conflict's reread brings the fresh digest", async () => {
    let finishReread = () => {};
    const reread = vi
      .spyOn(QueryClient.prototype, "refetchQueries")
      .mockReturnValue(new Promise<void>(resolve => (finishReread = resolve)));
    const { result } = renderEditor(workspaceAgent());

    act(() =>
      result.current.onChange({ provider: "claude", model: "sonnet", reasoning_effort: "" })
    );
    await act(async () => saves[0]!.reject(new AgentDigestConflictError("stale")));

    expect(result.current.isPending).toBe(true);
    expect(result.current.conflictMessage).toBe(
      "This agent changed somewhere else. Pick the model again."
    );
    await act(async () => finishReread());
    await waitFor(() => expect(result.current.isPending).toBe(false));
    reread.mockRestore();
  });

  it("Should restore server truth and show the error when a save fails", async () => {
    const { result } = renderEditor(workspaceAgent());

    act(() =>
      result.current.onChange({ provider: "claude", model: "sonnet", reasoning_effort: "" })
    );
    await act(async () => saves[0]!.reject(new Error("disk full")));

    expect(result.current.value.model).toBe("");
    expect(result.current.error).toBe("disk full");
  });

  it("Should drop the agent's own runtime when it goes back to the default", () => {
    const { result } = renderEditor(
      workspaceAgent({
        provider: "claude",
        model: "sonnet",
        reasoning_effort: "high",
        speed: "fast",
      })
    );
    expect(result.current.overridden).toBe(true);

    act(() => result.current.onUseDefault());

    expect(result.current.overridden).toBe(false);
    // The server resolves the default's speed; the current one stays until the reread.
    expect(result.current.speed).toBe("fast");
    expect(result.current.isPending).toBe(true);
    const { agent } = savedParams(0).params;
    expect(agent).not.toHaveProperty("provider");
    expect(agent).not.toHaveProperty("model");
    expect(agent).not.toHaveProperty("reasoning_effort");
    expect(agent).not.toHaveProperty("speed");
    expect(agent.acp_options).toEqual([]);
  });
});
