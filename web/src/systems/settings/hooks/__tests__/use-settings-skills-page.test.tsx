import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { storyCompany } from "@/storybook/fintech-scenario";

vi.mock("@tanstack/react-router", () => ({
  useMatchRoute: () => () => false,
}));

vi.mock("@/systems/settings/adapters/settings-api", () => ({
  getSettingsRestartStatus: vi.fn(),
  getSettingsSkills: vi.fn(),
  updateSettingsSkills: vi.fn(),
  triggerSettingsRestart: vi.fn(),
  SettingsApiError: class SettingsApiError extends Error {
    status = 500;
  },
}));

import { getSettingsSkills, updateSettingsSkills } from "@/systems/settings/adapters/settings-api";
import { initialSettingsRestartState } from "@/systems/settings/stores/settings-restart-store";
import { useSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import type { SettingsSkillsSection } from "@/systems/settings";
import { useSettingsSkillsPage } from "../use-settings-skills-page";

const skillsEnvelope: SettingsSkillsSection = {
  section: "skills",
  scope: "global",
  available_scopes: ["global"],
  runtime_available: true,
  discovered_count: 10,
  disabled_count: 1,
  config: {
    enabled: true,
    disabled_skills: ["alpha"],
    poll_interval: "5m",
    marketplace: {
      registry: "agh",
      base_url: storyCompany.registryBaseUrl,
    },
    allowed_marketplace_mcp: [],
    allowed_marketplace_hooks: [],
  },
  links: [{ label: "skills", path: "/marketplace/skills?tab=installed" }],
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
  vi.mocked(getSettingsSkills).mockResolvedValue(skillsEnvelope);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSettingsSkillsPage", () => {
  it("loads the envelope and seeds the draft with the current config", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });

    await waitFor(() => {
      expect(result.current.envelope).toBeTruthy();
      expect(result.current.draft).toEqual(skillsEnvelope.config);
    });
  });

  it("marks disabled dirty independently from policy dirty when toggling a disabled skill", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.toggleDisabled("beta");
    });

    expect(result.current.isDisabledDirty).toBe(true);
    expect(result.current.isPolicyDirty).toBe(false);

    act(() => {
      result.current.handleResetDisabled();
    });

    expect(result.current.isDisabledDirty).toBe(false);
  });

  it("marks policy dirty independently when a marketplace field changes", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.setDraft({
        ...skillsEnvelope.config,
        marketplace: { ...skillsEnvelope.config.marketplace, registry: "other" },
      });
    });

    expect(result.current.isPolicyDirty).toBe(true);
    expect(result.current.isDisabledDirty).toBe(false);
  });

  it("save disabled sends full config with only disabled_skills changed and records applied-now label", async () => {
    vi.mocked(updateSettingsSkills).mockResolvedValue({
      section: "skills",
      scope: "global",
      applied: true,
      active_config_hash: "sha256:test-active",
      active_generation: 1,
      apply_record_id: "cfg_apply_test",
      lifecycle: "live",
      next_action: "none",
      restart_required: false,
      write_target: "global-config",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.toggleDisabled("beta");
    });
    act(() => {
      result.current.handleSaveDisabled();
    });

    await waitFor(() => {
      expect(result.current.lastDisabledLabel).toContain("applied immediately");
    });
    expect(updateSettingsSkills).toHaveBeenCalledWith(
      {
        config: expect.objectContaining({
          disabled_skills: expect.arrayContaining(["alpha", "beta"]),
          marketplace: skillsEnvelope.config.marketplace,
        }),
      },
      { scope: "global" }
    );
  });

  it("save policy sends full config with only policy changes and records restart-required label", async () => {
    vi.mocked(updateSettingsSkills).mockResolvedValue({
      section: "skills",
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
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.setDraft({
        ...skillsEnvelope.config,
        poll_interval: "10m",
      });
    });
    act(() => {
      result.current.handleSavePolicy();
    });

    await waitFor(() => {
      expect(result.current.lastPolicyLabel).toContain("restart required");
    });
    expect(updateSettingsSkills).toHaveBeenCalledWith(
      {
        config: expect.objectContaining({
          poll_interval: "10m",
          disabled_skills: skillsEnvelope.config.disabled_skills,
        }),
      },
      { scope: "global" }
    );
  });
});
