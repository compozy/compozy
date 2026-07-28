import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
  useMatchRoute: () => () => false,
}));

vi.mock("@/systems/settings/adapters/settings-api", () => ({
  getSettingsRestartStatus: vi.fn(),
  getSettingsNetwork: vi.fn(),
  updateSettingsNetwork: vi.fn(),
  triggerSettingsRestart: vi.fn(),
  SettingsApiError: class SettingsApiError extends Error {
    status = 500;
  },
}));

import {
  getSettingsNetwork,
  updateSettingsNetwork,
} from "@/systems/settings/adapters/settings-api";
import { initialSettingsRestartState } from "@/systems/settings/stores/settings-restart-store";
import { useSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import type { SettingsNetworkSection } from "@/systems/settings";
import { settingsNetworkSectionFixture } from "@/systems/settings/mocks";
import { useSettingsNetworkPage } from "../use-settings-network-page";

const networkEnvelope: SettingsNetworkSection = structuredClone(settingsNetworkSectionFixture);

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
  useSettingsRestartStore.setState({
    ...initialSettingsRestartState,
    startRestart: useSettingsRestartStore.getState().startRestart,
    updateRestart: useSettingsRestartStore.getState().updateRestart,
    clearRestart: useSettingsRestartStore.getState().clearRestart,
    recordMutation: useSettingsRestartStore.getState().recordMutation,
  });
  vi.mocked(getSettingsNetwork).mockResolvedValue(networkEnvelope);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSettingsNetworkPage", () => {
  it("loads the envelope and seeds the draft", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsNetworkPage(), { wrapper });

    await waitFor(() => {
      expect(result.current.envelope).toBeTruthy();
      expect(result.current.draft).toEqual(networkEnvelope.config);
    });
  });

  it("marks the page dirty when the draft diverges and resets on discard", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsNetworkPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.setDraft({ ...networkEnvelope.config, max_replay_age: 450 });
    });
    expect(result.current.isDirty).toBe(true);

    act(() => {
      result.current.handleReset();
    });
    expect(result.current.isDirty).toBe(false);
  });

  it("save persists draft via the mutation and stores the restart-required applied label", async () => {
    vi.mocked(updateSettingsNetwork).mockResolvedValue({
      section: "network",
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
    const { result } = renderHook(() => useSettingsNetworkPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    const updatedConfig = {
      ...networkEnvelope.config,
      live: {
        ...networkEnvelope.config.live,
        defaults: { ...networkEnvelope.config.live.defaults, max_wakes: 12 },
      },
    };
    act(() => result.current.setDraft(updatedConfig));
    await waitFor(() => expect(result.current.isDirty).toBe(true));
    act(() => result.current.handleSave());

    await waitFor(() => {
      expect(result.current.lastAppliedLabel).toContain("restart required");
    });
    expect(updateSettingsNetwork).toHaveBeenCalledWith({ config: updatedConfig });
  });
});
