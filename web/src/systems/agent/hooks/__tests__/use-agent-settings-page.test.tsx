import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { primaryAgentFixture } from "@/systems/agent/testing";

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  updateMutate: vi.fn(),
  refetch: vi.fn(),
  toastSuccess: vi.fn(),
}));

vi.mock("@tanstack/react-router", () => ({ useNavigate: () => mocks.navigate }));
vi.mock("sonner", () => ({ toast: { success: mocks.toastSuccess } }));
vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "ws-test", activeWorkspace: { name: "Test" } }),
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
    data: { ...primaryAgentFixture, origin: "workspace" },
    isLoading: false,
    error: null,
    refetch: mocks.refetch,
  }),
  useUpdateAgent: () => ({ mutate: mocks.updateMutate, isPending: false }),
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
    act(() => result.current.onSave());

    const [variables, callbacks] = mocks.updateMutate.mock.calls.at(-1)!;
    expect(variables).toMatchObject({
      name: primaryAgentFixture.name,
      params: {
        expected_digest: primaryAgentFixture.definition_digest,
        workspace: "ws-test",
        agent: { prompt: "Updated prompt" },
      },
    });

    act(() =>
      callbacks.onSuccess({ ...primaryAgentFixture, origin: "workspace", prompt: "Updated prompt" })
    );
    expect(mocks.toastSuccess).toHaveBeenCalledWith("Changes saved");
    expect(result.current.dirty).toBe(false);
  });

  it("Should render conflict recovery and reload the fresh digest before retry", async () => {
    const { result } = renderHook(() =>
      useAgentSettingsPage({ name: primaryAgentFixture.name, section: "instructions" })
    );
    await waitFor(() => expect(result.current.draft).not.toBeNull());
    act(() => result.current.patchDraft({ prompt: "Conflicting prompt" }));
    act(() => result.current.onSave());
    const callbacks = mocks.updateMutate.mock.calls.at(-1)![1];
    act(() => callbacks.onError(new AgentDigestConflictError("stale")));

    expect(result.current.conflictBanner).toMatch(/changed elsewhere/i);
    await act(async () => result.current.onReloadAndRetry());
    expect(mocks.refetch).toHaveBeenCalledTimes(1);
    expect(result.current.draft?.prompt).toBe("Reloaded prompt");
  });

  it("Should keep Save focusable-but-blocked with a caption after a permission denial", async () => {
    const { result } = renderHook(() =>
      useAgentSettingsPage({ name: primaryAgentFixture.name, section: "instructions" })
    );
    await waitFor(() => expect(result.current.draft).not.toBeNull());
    act(() => result.current.patchDraft({ prompt: "Denied prompt" }));
    act(() => result.current.onSave());
    const callbacks = mocks.updateMutate.mock.calls.at(-1)![1];
    act(() => callbacks.onError(new AgentApiError("forbidden", 403)));

    expect(result.current.mutationDenied).toBe(true);
    expect(result.current.saveBlocked).toBe(true);
    expect(result.current.saveBlockedCaption).toBe("Editing is not permitted for this agent.");
    expect(result.current.fieldsReadOnly).toBe(true);
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
});
