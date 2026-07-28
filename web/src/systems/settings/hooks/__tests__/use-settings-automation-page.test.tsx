import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
  useMatchRoute: () => () => false,
}));

vi.mock("@/systems/settings/adapters/settings-api", () => ({
  getSettingsRestartStatus: vi.fn(),
  getSettingsAutomation: vi.fn(),
  updateSettingsAutomation: vi.fn(),
  triggerSettingsRestart: vi.fn(),
  SettingsApiError: class SettingsApiError extends Error {
    status = 500;
  },
}));

import {
  getSettingsAutomation,
  updateSettingsAutomation,
} from "@/systems/settings/adapters/settings-api";
import { initialSettingsRestartState } from "@/systems/settings/stores/settings-restart-store";
import { useSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import type { SettingsAutomationSection } from "@/systems/settings";
import { useSettingsAutomationPage } from "../use-settings-automation-page";

const automationEnvelope: SettingsAutomationSection = {
  section: "automation",
  scope: "global",
  available_scopes: ["global"],
  config: {
    enabled: true,
    timezone: "UTC",
    max_concurrent_jobs: 4,
    default_fire_limit: { max: 5, window: "1m" },
  },
  runtime: {
    available: true,
    running: true,
    scheduler_running: true,
    job_enabled: 2,
    job_total: 3,
    trigger_enabled: 1,
    trigger_total: 2,
  },
  links: [
    { label: "jobs", path: "/jobs" },
    { label: "triggers", path: "/triggers" },
  ],
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
  useSettingsRestartStore.setState({
    ...initialSettingsRestartState,
    startRestart: useSettingsRestartStore.getState().startRestart,
    updateRestart: useSettingsRestartStore.getState().updateRestart,
    clearRestart: useSettingsRestartStore.getState().clearRestart,
    recordMutation: useSettingsRestartStore.getState().recordMutation,
  });
  vi.mocked(getSettingsAutomation).mockResolvedValue(automationEnvelope);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSettingsAutomationPage", () => {
  it("loads the envelope and seeds the draft", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsAutomationPage(), { wrapper });

    await waitFor(() => {
      expect(result.current.envelope).toBeTruthy();
      expect(result.current.draft).toEqual(automationEnvelope.config);
    });
  });

  it("marks the page dirty when the draft diverges and resets on discard", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsAutomationPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.setDraft({ ...automationEnvelope.config, max_concurrent_jobs: 16 });
    });
    expect(result.current.isDirty).toBe(true);

    act(() => {
      result.current.handleReset();
    });
    expect(result.current.isDirty).toBe(false);
  });

  it("save persists draft via the mutation and stores the restart-required applied label", async () => {
    vi.mocked(updateSettingsAutomation).mockResolvedValue({
      section: "automation",
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
    const { result } = renderHook(() => useSettingsAutomationPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.setDraft({ ...automationEnvelope.config, timezone: "America/Sao_Paulo" });
      result.current.handleSave();
    });

    await waitFor(() => {
      expect(result.current.lastAppliedLabel).toContain("restart required");
    });
    expect(updateSettingsAutomation).toHaveBeenCalled();
  });
});
