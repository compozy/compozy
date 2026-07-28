import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
  useMatchRoute: () => () => false,
}));

vi.mock("@/systems/settings/adapters/settings-api", () => ({
  getSettingsRestartStatus: vi.fn(),
  getSettingsMemory: vi.fn(),
  updateSettingsMemory: vi.fn(),
  triggerSettingsRestart: vi.fn(),
  SettingsApiError: class SettingsApiError extends Error {
    status = 500;
  },
}));

vi.mock("@/systems/knowledge", () => ({
  useTriggerMemoryDream: () => triggerDreamMock,
}));

const triggerDreamMock = {
  mutate: vi.fn(),
  isPending: false,
};

import { getSettingsMemory, updateSettingsMemory } from "@/systems/settings/adapters/settings-api";
import { settingsMemoryConfigFixture } from "@/systems/settings/mocks/fixtures";
import { initialSettingsRestartState } from "@/systems/settings/stores/settings-restart-store";
import { useSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import type { SettingsMemorySection } from "@/systems/settings";
import { useSettingsMemoryPage } from "../use-settings-memory-page";

const memoryEnvelope: SettingsMemorySection = {
  section: "memory",
  scope: "global",
  available_scopes: ["global"],
  actions: {
    consolidate: { available: true, behavior: "action_trigger", name: "consolidate" },
  },
  config: settingsMemoryConfigFixture,
  health: {
    available: true,
    dream_enabled: true,
    file_count: 12,
    last_consolidated_at: "2026-04-17T10:00:00Z",
  },
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
  triggerDreamMock.mutate.mockReset();
  triggerDreamMock.isPending = false;
  useSettingsRestartStore.setState({
    ...initialSettingsRestartState,
    startRestart: useSettingsRestartStore.getState().startRestart,
    updateRestart: useSettingsRestartStore.getState().updateRestart,
    clearRestart: useSettingsRestartStore.getState().clearRestart,
    recordMutation: useSettingsRestartStore.getState().recordMutation,
  });
  vi.mocked(getSettingsMemory).mockResolvedValue(memoryEnvelope);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSettingsMemoryPage", () => {
  it("loads the envelope and seeds the draft with the current config", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsMemoryPage(), { wrapper });

    await waitFor(() => {
      expect(result.current.envelope).toBeTruthy();
      expect(result.current.draft).toEqual(memoryEnvelope.config);
    });
  });

  it("marks the page dirty when draft diverges from the envelope and resets on discard", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsMemoryPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.setDraft({
        ...memoryEnvelope.config,
        enabled: false,
      });
    });

    expect(result.current.isDirty).toBe(true);

    act(() => {
      result.current.handleReset();
    });

    expect(result.current.isDirty).toBe(false);
  });

  it("save persists draft via the mutation and stores the applied label", async () => {
    vi.mocked(updateSettingsMemory).mockResolvedValue({
      section: "memory",
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
    const { result } = renderHook(() => useSettingsMemoryPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.setDraft({
        ...memoryEnvelope.config,
        enabled: false,
      });
      result.current.handleSave();
    });

    await waitFor(() => {
      expect(result.current.lastAppliedLabel).toContain("restart required");
    });
    expect(updateSettingsMemory).toHaveBeenCalled();
  });

  it("exposes handleTriggerDream that delegates to the memory dreaming mutation", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsMemoryPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.handleTriggerDream();
    });

    expect(triggerDreamMock.mutate).toHaveBeenCalled();
  });
});
