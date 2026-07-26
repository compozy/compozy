import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
  useMatchRoute: () => () => false,
}));

vi.mock("@/systems/settings/adapters/settings-api", () => ({
  getSettingsObservability: vi.fn(),
  updateSettingsObservability: vi.fn(),
  getSettingsRestartStatus: vi.fn(),
  triggerSettingsRestart: vi.fn(),
  SettingsApiError: class SettingsApiError extends Error {
    status = 500;
  },
}));

import {
  getSettingsObservability,
  updateSettingsObservability,
} from "@/systems/settings/adapters/settings-api";
import { initialSettingsRestartState } from "@/systems/settings/stores/settings-restart-store";
import { useSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import type { SettingsObservabilitySection } from "@/systems/settings";
import { useSettingsObservabilityPage } from "../use-settings-observability-page";

const envelope: SettingsObservabilitySection = {
  section: "observability",
  scope: "global",
  available_scopes: ["global"],
  config: {
    enabled: true,
    max_global_bytes: 1024 * 1024 * 1024,
    retention_days: 7,
    transcripts: {
      enabled: true,
      max_bytes_per_session: 256 * 1024 * 1024,
      segment_bytes: 1024 * 1024,
    },
  },
  log_tail: {
    available: true,
    stream_url: "/api/settings/observability/log-tail",
    transport: "sse",
  },
  runtime: {
    active_agents: 1,
    active_sessions: 1,
    available: true,
    global_db_size_bytes: 180 * 1024 * 1024,
    session_db_size_bytes: 132 * 1024 * 1024,
    uptime_seconds: 60,
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
  useSettingsRestartStore.setState({
    ...initialSettingsRestartState,
    startRestart: useSettingsRestartStore.getState().startRestart,
    updateRestart: useSettingsRestartStore.getState().updateRestart,
    clearRestart: useSettingsRestartStore.getState().clearRestart,
    recordMutation: useSettingsRestartStore.getState().recordMutation,
  });
  vi.mocked(getSettingsObservability).mockResolvedValue(envelope);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSettingsObservabilityPage", () => {
  it("loads the envelope and seeds the draft", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsObservabilityPage(), { wrapper });

    await waitFor(() => {
      expect(result.current.envelope).toBeTruthy();
      expect(result.current.draft).toEqual(envelope.config);
    });
  });

  it("surfaces the save error when the mutation rejects", async () => {
    vi.mocked(updateSettingsObservability).mockRejectedValue(new Error("rejected by service"));

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsObservabilityPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.setDraft({
        ...envelope.config,
        retention_days: 3,
      });
      result.current.handleSave();
    });

    await waitFor(() => {
      expect(result.current.saveError).toBe("rejected by service");
    });
  });
});
