import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const navigateMock = vi.fn();

vi.mock("@tanstack/react-router", () => ({
  useMatchRoute: () => () => false,
  useNavigate: () => navigateMock,
}));

vi.mock("@/systems/settings/adapters/settings-api", () => ({
  getSettingsRestartStatus: vi.fn(),
  listSettingsSandboxes: vi.fn(),
  putSettingsSandbox: vi.fn(),
  deleteSettingsSandbox: vi.fn(),
  triggerSettingsRestart: vi.fn(),
  SettingsApiError: class SettingsApiError extends Error {
    status = 500;
    constructor(message: string, status: number) {
      super(message);
      this.status = status;
    }
  },
}));

import {
  deleteSettingsSandbox,
  listSettingsSandboxes,
  putSettingsSandbox,
  SettingsApiError,
} from "@/systems/settings/adapters/settings-api";
import { initialSettingsRestartState } from "@/systems/settings/stores/settings-restart-store";
import { useSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import type { SettingsSandboxCollection } from "@/systems/settings";
import { useSandboxPage } from "@/systems/sandbox/hooks/use-sandbox-page";

const localEnv: SettingsSandboxCollection["sandboxes"][number] = {
  name: "local",
  workspace_usage_count: 3,
  profile: {
    backend: "local",
    sync_mode: "none",
    persistence: "transient",
    runtime_root: "~",
    env: { NODE_ENV: "development" },
  },
  source_metadata: {
    available_targets: ["global-config"],
    effective_source: { kind: "global-config", scope: "global" },
  },
};

const daytonaEnv: SettingsSandboxCollection["sandboxes"][number] = {
  name: "daytona-staging",
  workspace_usage_count: 1,
  profile: {
    backend: "daytona",
    sync_mode: "session-bidirectional",
    persistence: "reuse",
    runtime_root: "/workspace",
    network: { allow_public_ingress: true },
    daytona: { api_url: "https://daytona.dev", target: "staging" },
  },
  source_metadata: {
    available_targets: ["global-config", "workspace-config"],
    effective_source: {
      kind: "workspace-config",
      scope: "workspace",
      workspace_id: "ws_alpha",
    },
    shadowed_sources: [{ kind: "global-config", scope: "global" }],
  },
};

const collection: SettingsSandboxCollection = {
  collection: "sandboxes",
  scope: "global",
  available_scopes: ["global"],
  sandboxes: [localEnv, daytonaEnv],
};

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });

  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);

  return { queryClient, wrapper };
}

beforeEach(() => {
  vi.clearAllMocks();
  navigateMock.mockReset();
  useSettingsRestartStore.setState({
    ...initialSettingsRestartState,
    startRestart: useSettingsRestartStore.getState().startRestart,
    updateRestart: useSettingsRestartStore.getState().updateRestart,
    clearRestart: useSettingsRestartStore.getState().clearRestart,
    recordMutation: useSettingsRestartStore.getState().recordMutation,
  });
  vi.mocked(listSettingsSandboxes).mockResolvedValue(collection);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSandboxPage", () => {
  it("preserves the accepted e2b backend from route state", () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSandboxPage({ backend: "e2b" }), { wrapper });

    expect(result.current.backend).toBe("e2b");
    expect(result.current.hasActiveFilters).toBe(true);
  });

  it("Should write filters to route search and preserve cards view when clearing them", () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () =>
        useSandboxPage({
          backend: "daytona",
          persistence: "reuse",
          q: "staging",
          view: "cards",
        }),
      { wrapper }
    );
    const current = {
      backend: "daytona",
      persistence: "reuse",
      q: "staging",
      view: "cards" as const,
    };

    act(() => result.current.setQuery("  local  "));
    expect(navigateMock.mock.lastCall?.[0].search(current)).toEqual({ ...current, q: "local" });

    act(() => result.current.setBackend("local"));
    expect(navigateMock.mock.lastCall?.[0].search(current)).toEqual({
      ...current,
      backend: "local",
    });

    act(() => result.current.setPersistence("transient"));
    expect(navigateMock.mock.lastCall?.[0].search(current)).toEqual({
      ...current,
      persistence: "transient",
    });

    act(() => result.current.setView("rows"));
    expect(navigateMock.mock.lastCall?.[0].search(current)).toEqual({
      ...current,
      view: undefined,
    });

    act(() => result.current.clearFilters());
    expect(navigateMock.mock.lastCall?.[0].search(current)).toEqual({ view: "cards" });
    expect(navigateMock.mock.lastCall?.[0].to).toBe("/sandbox");
  });

  it("computes total counts and workspace usage", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSandboxPage(), { wrapper });

    await waitFor(() => expect(result.current.sandboxes).toHaveLength(2));
    expect(result.current.counts).toEqual({ total: 2, totalWorkspaces: 4 });
  });

  it("seeds edit drafts from the selected entry without dropping it", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSandboxPage(), { wrapper });

    await waitFor(() => expect(result.current.sandboxes).toHaveLength(2));

    act(() => {
      result.current.openEdit(daytonaEnv);
    });

    expect(result.current.editor).toMatchObject({
      mode: "edit",
      name: "daytona-staging",
      draft: expect.objectContaining({
        backend: "daytona",
        sync_mode: "session-bidirectional",
        persistence: "reuse",
        runtime_root: "/workspace",
      }),
    });
  });

  it("submits the replace request preserving unedited nested profile keys", async () => {
    vi.mocked(putSettingsSandbox).mockResolvedValue({
      section: "general",
      scope: "global",
      applied: true,
      active_config_hash: "sha256:test-active",
      active_generation: 1,
      apply_record_id: "cfg_apply_test",
      lifecycle: "live",
      next_action: "none",
      restart_required: true,
      write_target: "global-config",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSandboxPage(), { wrapper });

    await waitFor(() => expect(result.current.sandboxes).toHaveLength(2));

    act(() => {
      result.current.openEdit(daytonaEnv);
    });
    act(() => {
      result.current.updateDraft(draft => ({ ...draft, runtime_root: "/home/agh" }));
    });
    act(() => {
      result.current.saveEditor();
    });

    await waitFor(() => expect(result.current.lastAction?.kind).toBe("saved"));

    expect(putSettingsSandbox).toHaveBeenCalledWith("daytona-staging", {
      profile: expect.objectContaining({
        backend: "daytona",
        sync_mode: "session-bidirectional",
        persistence: "reuse",
        runtime_root: "/home/agh",
        network: { allow_public_ingress: true },
        daytona: { api_url: "https://daytona.dev", target: "staging" },
      }),
    });
  });

  it("rejects saves when the backend is empty", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSandboxPage(), { wrapper });

    await waitFor(() => expect(result.current.sandboxes).toHaveLength(2));

    act(() => {
      result.current.openEdit(daytonaEnv);
      result.current.updateDraft(draft => ({ ...draft, backend: "" }));
    });

    expect(result.current.editorIsValid).toBe(false);
  });

  it("rejects saves when a non-secret environment key is invalid", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSandboxPage(), { wrapper });

    await waitFor(() => expect(result.current.sandboxes).toHaveLength(2));

    act(() => {
      result.current.openEdit(localEnv);
    });
    act(() => {
      result.current.updateDraft(draft => ({
        ...draft,
        env: [{ key: "9INVALID", value: "ignored" }],
      }));
    });
    expect(result.current.editorIsValid).toBe(false);
    act(() => {
      result.current.saveEditor();
    });

    expect(putSettingsSandbox).not.toHaveBeenCalled();
  });

  it("records delete actions with the workspace usage count", async () => {
    vi.mocked(deleteSettingsSandbox).mockResolvedValue({
      section: "general",
      scope: "global",
      applied: true,
      active_config_hash: "sha256:test-active",
      active_generation: 1,
      apply_record_id: "cfg_apply_test",
      lifecycle: "live",
      next_action: "none",
      restart_required: true,
      write_target: "global-config",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSandboxPage(), { wrapper });

    await waitFor(() => expect(result.current.sandboxes).toHaveLength(2));

    act(() => {
      result.current.openDelete(localEnv);
    });
    act(() => {
      result.current.confirmDelete();
    });

    await waitFor(() => expect(result.current.lastAction?.kind).toBe("deleted"));
    expect(deleteSettingsSandbox).toHaveBeenCalledWith("local");
    expect(result.current.lastAction).toMatchObject({ name: "local", usageCount: 3 });
  });

  it("keeps the delete target open on conflict errors", async () => {
    vi.mocked(deleteSettingsSandbox).mockRejectedValue(
      new SettingsApiError("sandbox still referenced", 409)
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSandboxPage(), { wrapper });

    await waitFor(() => expect(result.current.sandboxes).toHaveLength(2));

    act(() => {
      result.current.openDelete(localEnv);
    });
    act(() => {
      result.current.confirmDelete();
    });

    await waitFor(() => expect(result.current.deleteError).toBe("sandbox still referenced"));
    expect(result.current.deleteTarget.mode).toBe("open");
  });

  it("filters profiles client-side by search query and backend", async () => {
    const { wrapper } = createWrapper();
    const { result, rerender } = renderHook(
      ({ search }: { search?: Parameters<typeof useSandboxPage>[0] }) => useSandboxPage(search),
      { wrapper, initialProps: { search: {} } }
    );

    await waitFor(() => expect(result.current.sandboxes).toHaveLength(2));
    expect(result.current.filtered).toHaveLength(2);

    rerender({ search: { q: "daytona" } });
    expect(result.current.filtered.map(entry => entry.name)).toEqual(["daytona-staging"]);

    rerender({ search: { backend: "local" } });
    expect(result.current.filtered.map(entry => entry.name)).toEqual(["local"]);
  });

  it("selects a profile for the detail sheet", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSandboxPage(), { wrapper });

    await waitFor(() => expect(result.current.sandboxes).toHaveLength(2));

    act(() => {
      result.current.openInspect(localEnv);
    });
    expect(result.current.selectedEntry?.name).toBe("local");

    act(() => {
      result.current.closeInspect();
    });
    expect(result.current.selectedEntry).toBeNull();
  });
});
