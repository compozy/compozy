import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

let matchedRoutes: Record<string, boolean> = {};

vi.mock("@tanstack/react-router", () => ({
  useMatchRoute: () => (opts: { to: string; fuzzy?: boolean }) => matchedRoutes[opts.to] ?? false,
}));

vi.mock("@/systems/settings/adapters/settings-api", () => ({
  getSettingsRestartStatus: vi.fn(),
  triggerSettingsRestart: vi.fn(),
}));

import { getSettingsRestartStatus } from "@/systems/settings/adapters/settings-api";
import { settingsRestartStore } from "@/systems/settings/stores/settings-restart-store";
import { resetSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import { useSettingsPage } from "../use-settings-page";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });

  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);

  return { queryClient, wrapper };
}

beforeEach(() => {
  matchedRoutes = {};
  vi.clearAllMocks();
  resetSettingsRestartStore();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSettingsPage", () => {
  it("exposes the Settings sections and root path", () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsPage(), { wrapper });

    expect(result.current.sections.map(section => section.slug)).toEqual([
      "general",
      "appearance",
      "layouts",
      "providers",
      "memory",
      "roles",
      "skills",
      "automation",
      "network",
      "gateway",
      "attention",
      "observability",
      "hooks",
      "extensions",
    ]);
    expect(result.current.rootPath).toBe("/settings");
    expect(result.current.sectionPath("general")).toBe("/settings/general");
    expect(result.current.activeSectionSlug).toBeNull();
  });

  it("resolves the active section from the router match", () => {
    matchedRoutes["/settings/memory"] = true;
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useSettingsPage(), { wrapper });

    expect(result.current.activeSectionSlug).toBe("memory");
    expect(result.current.activeSection?.label).toBe("Memory");
  });

  it("prefers an explicit slug over the router match", () => {
    matchedRoutes["/settings/general"] = true;
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useSettingsPage({ currentSlug: "skills" }), {
      wrapper,
    });

    expect(result.current.activeSectionSlug).toBe("skills");
  });

  it("keeps the restart notice hidden when no mutation is pending", () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsPage(), { wrapper });

    expect(result.current.restart.isVisible).toBe(false);
    expect(result.current.restart.isRestartRequired).toBe(false);
    expect(result.current.restart.status).toBeNull();
    expect(result.current.restart).not.toHaveProperty("isPolling");
    expect(result.current.restart).not.toHaveProperty("isSuccessful");
    expect(result.current.restart).not.toHaveProperty("isFailed");
  });

  it("shows the notice and polling state once restart begins", async () => {
    vi.mocked(getSettingsRestartStatus).mockResolvedValue({
      operation_id: "op_page",
      status: "stopping",
      old_pid: 123,
      old_socket_path: "/tmp/compozy.sock",
      old_started_at: "2026-04-17T10:00:00Z",
      active_session_count: 1,
      started_at: "2026-04-17T10:05:00Z",
      updated_at: "2026-04-17T10:05:01Z",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsPage(), { wrapper });

    act(() => {
      settingsRestartStore.trigger.settingsMutationRecorded({
        mutation: {
          section: "general",
          restartRequired: true,
          warnings: [],
          completedAt: new Date().toISOString(),
        },
      });
      settingsRestartStore.trigger.restartOperationStarted({
        operationId: "op_page",
      });
    });

    await waitFor(() => {
      expect(result.current.restart.status).toBe("stopping");
    });

    expect(result.current.restart.isVisible).toBe(true);
    expect(result.current.restart.operationId).toBe("op_page");
    expect(result.current.restart.isRestartRequired).toBe(true);
  });
});
