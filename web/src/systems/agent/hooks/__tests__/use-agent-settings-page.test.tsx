import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import type { PropsWithChildren } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { primaryAgentFixture } from "@/systems/agent/testing";
import { resetProfileViews, setProfileView, useProfileReadScope } from "@/systems/profiles";

const mocks = vi.hoisted(() => ({
  agent: undefined as unknown,
  navigate: vi.fn(),
  updateMutate: vi.fn(),
  updateMutateAsync: vi.fn(),
  refetch: vi.fn(),
  toastSuccess: vi.fn(),
  workspaceId: "ws-test",
  guard: undefined as { onDiscard?: () => void } | undefined,
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
  useUnsavedGuard: (options: { onDiscard?: () => void }) => {
    mocks.guard = options;
    return { confirmDialog: null };
  },
}));

import { AgentApiError, AgentDigestConflictError } from "../../adapters/agent-api";
import { agentKeys } from "../../lib/query-keys";
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

  // Invariant (#1653): a page that goes away holding edits without Save or Discard (a command
  //   from outside the client, a project switch) leaves them to its next mount, which saves them
  //   against the digest they were read from; Save and the guard's Discard drop them.
  describe("unsaved edits kept across mounts [#1653]", () => {
    const page = () =>
      useAgentSettingsPage({ name: primaryAgentFixture.name, section: "instructions" });

    it("Should bring the edits back unsaved and meet a newer version as a conflict", async () => {
      const first = renderHook(page, { wrapper: createWrapper() });
      act(() => first.result.current.patchDraft({ prompt: "Kept prompt" }));
      first.unmount();

      mocks.agent = {
        ...primaryAgentFixture,
        origin: "workspace",
        definition_digest: "newer-digest",
        prompt: "Their prompt",
      };
      const second = renderHook(page, { wrapper: createWrapper() });
      expect(second.result.current.dirty).toBe(true);
      expect(second.result.current.draft?.prompt).toBe("Kept prompt");

      mocks.updateMutateAsync.mockRejectedValueOnce(new AgentDigestConflictError("stale"));
      act(() => second.result.current.onSave());
      expect(mocks.updateMutateAsync.mock.calls.at(-1)![0]).toMatchObject({
        params: { expected_digest: primaryAgentFixture.definition_digest },
      });
      await waitFor(() => expect(second.result.current.phase).toBe("conflict"));

      act(() => mocks.guard?.onDiscard?.());
      expect(second.result.current.dirty).toBe(false);
      second.unmount();

      const third = renderHook(page, { wrapper: createWrapper() });
      expect(third.result.current.dirty).toBe(false);
      expect(third.result.current.draft?.prompt).toBe("Their prompt");
    });

    it("Should keep one draft per editor of the same agent", () => {
      const first = renderHook(page, { wrapper: createWrapper() });
      const second = renderHook(page, { wrapper: createWrapper() });
      act(() => first.result.current.patchDraft({ prompt: "First window" }));
      act(() => second.result.current.patchDraft({ prompt: "Second window" }));
      first.unmount();
      second.unmount();

      const newest = renderHook(page, { wrapper: createWrapper() });
      expect(newest.result.current.draft?.prompt).toBe("Second window");
      act(() => mocks.guard?.onDiscard?.());
      newest.unmount();

      const older = renderHook(page, { wrapper: createWrapper() });
      expect(older.result.current.dirty).toBe(true);
      expect(older.result.current.draft?.prompt).toBe("First window");
    });

    it("Should resume each agent's draft when a mounted editor moves between agents", () => {
      const { result, rerender } = renderHook(
        ({ name }: { name: string }) => useAgentSettingsPage({ name, section: "instructions" }),
        { initialProps: { name: "agent-a" }, wrapper: createWrapper() }
      );
      act(() => result.current.patchDraft({ prompt: "Edits to A" }));

      rerender({ name: "agent-b" });
      expect(result.current.dirty).toBe(false);
      act(() => result.current.patchDraft({ prompt: "Edits to B" }));
      // Discarding B's edits leaves the draft kept for A alone.
      act(() => mocks.guard?.onDiscard?.());
      expect(result.current.dirty).toBe(false);

      rerender({ name: "agent-a" });
      expect(result.current.dirty).toBe(true);
      expect(result.current.draft?.prompt).toBe("Edits to A");
    });

    it("Should not keep edits a save running at unmount went on to write", async () => {
      let finish!: (agent: unknown) => void;
      mocks.updateMutateAsync.mockReturnValueOnce(
        new Promise(resolve => {
          finish = resolve;
        })
      );
      const first = renderHook(page, { wrapper: createWrapper() });
      act(() => first.result.current.patchDraft({ prompt: "Saved while closing" }));
      act(() => first.result.current.onSave());
      first.unmount();

      await act(async () =>
        finish({ ...primaryAgentFixture, origin: "workspace", prompt: "Saved while closing" })
      );
      const second = renderHook(page, { wrapper: createWrapper() });
      expect(second.result.current.dirty).toBe(false);
    });

    it("Should not bring back edits that were saved", async () => {
      const first = renderHook(page, { wrapper: createWrapper() });
      act(() => first.result.current.patchDraft({ prompt: "Saved prompt" }));
      mocks.updateMutateAsync.mockResolvedValueOnce({
        ...primaryAgentFixture,
        origin: "workspace",
        prompt: "Saved prompt",
      });
      act(() => first.result.current.onSave());
      await waitFor(() => expect(first.result.current.dirty).toBe(false));
      first.unmount();

      const second = renderHook(page, { wrapper: createWrapper() });
      expect(second.result.current.dirty).toBe(false);
    });
  });
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

  // The page reads the agent through the detail query; the conflict reread refreshes that query.
  function renderQueryBacked(server: { agent: unknown; offline: boolean }) {
    return renderHook(
      () => {
        const { destination } = useProfileReadScope();
        const { data } = useQuery({
          queryKey: agentKeys.detail(primaryAgentFixture.name, "ws-test", destination),
          queryFn: () =>
            server.offline ? Promise.reject(new Error("offline")) : Promise.resolve(server.agent),
        });
        return { agent: data, editor: useAgentRuntimeEditor({ agent: data as never }) };
      },
      { wrapper: createWrapper() }
    );
  }

  it("Should hold picks until a conflict's reread brings the fresh digest", async () => {
    const server = { agent: workspaceAgent() as unknown, offline: false };
    const { result } = renderQueryBacked(server);
    await waitFor(() => expect(result.current.agent).toBeDefined());

    act(() =>
      result.current.editor.onChange({ provider: "claude", model: "sonnet", reasoning_effort: "" })
    );
    server.agent = workspaceAgent({ definition_digest: "fresh-digest" });
    await act(async () => saves[0]!.reject(new AgentDigestConflictError("stale")));
    expect(result.current.editor.conflictMessage).toBe(
      "This agent changed somewhere else. Pick the model again."
    );
    await waitFor(() => expect(result.current.editor.isPending).toBe(false));

    act(() =>
      result.current.editor.onChange({ provider: "claude", model: "opus", reasoning_effort: "" })
    );
    expect(savedParams(1).params).toMatchObject({ expected_digest: "fresh-digest" });
  });

  it("Should keep picks held when the conflict reread fails, until a retry succeeds", async () => {
    const server = { agent: workspaceAgent() as unknown, offline: false };
    const { result } = renderQueryBacked(server);
    await waitFor(() => expect(result.current.agent).toBeDefined());

    act(() =>
      result.current.editor.onChange({ provider: "claude", model: "sonnet", reasoning_effort: "" })
    );
    server.offline = true;
    await act(async () => saves[0]!.reject(new AgentDigestConflictError("stale")));
    await waitFor(() =>
      expect(result.current.editor.error).toBe("Couldn't load this agent's latest version.")
    );
    expect(result.current.editor.isPending).toBe(true);
    // A pick from the still-open popup waits too; it would only hit the stale digest again.
    act(() =>
      result.current.editor.onChange({ provider: "claude", model: "haiku", reasoning_effort: "" })
    );
    expect(mocks.updateMutateAsync).toHaveBeenCalledOnce();

    server.offline = false;
    server.agent = workspaceAgent({ definition_digest: "fresh-digest" });
    await act(async () => result.current.editor.onReload!());
    await waitFor(() => expect(result.current.editor.isPending).toBe(false));
    expect(result.current.editor.onReload).toBeNull();

    act(() =>
      result.current.editor.onChange({ provider: "claude", model: "opus", reasoning_effort: "" })
    );
    expect(savedParams(1).params).toMatchObject({ expected_digest: "fresh-digest" });
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

    // The saved override still shows until the return to the default lands.
    expect(result.current.overridden).toBe(true);
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

  it("Should name what changed when each save lands", async () => {
    const { result } = renderEditor(workspaceAgent({ provider: "claude", model: "sonnet" }));

    act(() => result.current.onSpeedChange("fast"));
    await act(async () => saves[0]!.resolve(workspaceAgent({ speed: "fast" })));
    expect(mocks.toastSuccess).toHaveBeenLastCalledWith("Fast turned on");

    act(() => result.current.onUseDefault());
    await act(async () => saves[1]!.resolve(workspaceAgent()));
    expect(mocks.toastSuccess).toHaveBeenLastCalledWith("Back to your default");
  });
});
