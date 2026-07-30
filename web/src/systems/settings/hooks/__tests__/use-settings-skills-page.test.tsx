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

vi.mock("@/systems/agent/adapters/agent-api", () => ({
  fetchAgents: vi.fn(),
}));

vi.mock("@/systems/workspace/adapters/workspace-api", () => ({
  fetchWorkspaces: vi.fn(),
}));

import { fetchAgents } from "@/systems/agent/adapters/agent-api";
import { primaryAgentFixture } from "@/systems/agent/mocks";
import { getSettingsSkills, updateSettingsSkills } from "@/systems/settings/adapters/settings-api";
import { resetSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import type { SettingsSkillsSection } from "@/systems/settings";
import { fetchWorkspaces } from "@/systems/workspace/adapters/workspace-api";
import { useSettingsSkillsPage } from "../use-settings-skills-page";
import { settingsSkillsDraftLogic } from "../settings-skills-draft-logic";

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
      registry: "compozy",
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
  resetSettingsRestartStore();
  vi.mocked(fetchAgents).mockResolvedValue([]);
  vi.mocked(fetchWorkspaces).mockResolvedValue([]);
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

  it("composes consecutive functional draft updates from the current draft", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.setDraft(current =>
        current ? { ...current, poll_interval: "10m" } : current
      );
      result.current.setDraft(current => (current ? { ...current, enabled: false } : current));
    });

    expect(result.current.draft).toMatchObject({ enabled: false, poll_interval: "10m" });
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

  it("Should adopt the normalized policy after save without leaving a dirty draft", async () => {
    let persisted = false;
    const envelopeWithoutDisabled = {
      ...skillsEnvelope,
      config: { ...skillsEnvelope.config, disabled_skills: undefined },
    };
    vi.mocked(getSettingsSkills).mockImplementation(async () =>
      persisted
        ? {
            ...envelopeWithoutDisabled,
            config: { ...envelopeWithoutDisabled.config, poll_interval: "10m0s" },
          }
        : envelopeWithoutDisabled
    );
    vi.mocked(updateSettingsSkills).mockImplementation(async () => {
      persisted = true;
      return {
        section: "skills",
        scope: "global",
        applied: true,
        active_config_hash: "sha256:test-active",
        active_generation: 1,
        apply_record_id: "cfg_apply_test",
        lifecycle: "restart-required",
        next_action: "restart-daemon",
        restart_required: true,
        write_target: "global-config",
      };
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.setDraft(current =>
        current ? { ...current, poll_interval: "10m" } : current
      );
    });
    expect(result.current.isPolicyDirty).toBe(true);

    act(() => {
      result.current.handleSavePolicy();
    });

    await waitFor(() => {
      expect(result.current.draft?.poll_interval).toBe("10m0s");
      expect(result.current.isPolicyDirty).toBe(false);
    });
  });

  it("Should select the canonical general agent when entering agent scope", async () => {
    vi.mocked(fetchAgents).mockResolvedValue([
      { ...primaryAgentFixture, name: "code_implementer" },
      { ...primaryAgentFixture, name: "general" },
    ]);
    vi.mocked(getSettingsSkills).mockImplementation(async filter => {
      const requested = filter ?? {};
      return {
        ...skillsEnvelope,
        scope: requested.scope ?? "global",
        available_scopes: ["global", "agent"],
        agent_name: requested.agent_name,
      };
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });

    await waitFor(() => expect(result.current.agents).toHaveLength(2));

    act(() => {
      result.current.selectAgentScope();
    });

    await waitFor(() => {
      expect(result.current.selection).toEqual({ scope: "agent", agentName: "general" });
    });
    expect(vi.mocked(getSettingsSkills).mock.calls.at(-1)?.[0]).toEqual({
      scope: "agent",
      agent_name: "general",
      workspace_id: undefined,
    });
  });

  it("Should replace a skills draft when its server scope changes", () => {
    const global = {
      enabled: true,
      marketplace: { registry: "https://registry.example" },
      poll_interval: "5m",
    };
    const agent = { ...global, enabled: false };
    const globalStore = settingsSkillsDraftLogic.createStore({ baseline: global, key: "global" });

    globalStore.trigger.draftChanged({ update: { ...global, enabled: false } });
    const agentStore = settingsSkillsDraftLogic.createStore({
      baseline: agent,
      key: "agent:ship",
      previous: globalStore.getSnapshot().context,
    });

    expect(agentStore.getSnapshot().context.draft).toEqual(agent);
    expect(agentStore.getSnapshot().context.labels).toEqual({ disabled: null, policy: null });
  });

  it("Should merge out-of-order independent saves into one baseline", async () => {
    let resolveDisabled!: () => void;
    let resolvePolicy!: () => void;
    const disabledSave = new Promise<void>(resolve => {
      resolveDisabled = resolve;
    });
    const policySave = new Promise<void>(resolve => {
      resolvePolicy = resolve;
    });
    const disabledBaseline = {
      ...skillsEnvelope.config,
      disabled_skills: ["alpha", "beta"],
    };
    const policyBaseline = {
      ...skillsEnvelope.config,
      poll_interval: "10m",
    };
    const store = settingsSkillsDraftLogic.createStore({
      baseline: skillsEnvelope.config,
      key: "global",
    });

    store.trigger.saveRequested({
      baseline: disabledBaseline,
      execute: () => disabledSave,
      kind: "disabled",
      label: "Disabled saved",
    });
    store.trigger.saveRequested({
      baseline: policyBaseline,
      execute: () => policySave,
      kind: "policy",
      label: "Policy saved",
    });
    resolvePolicy();
    await vi.waitFor(() => {
      expect(store.getSnapshot().context.baseline?.poll_interval).toBe("10m");
    });
    resolveDisabled();
    await vi.waitFor(() => {
      expect(store.getSnapshot().context.baseline).toMatchObject({
        disabled_skills: ["alpha", "beta"],
        poll_interval: "10m",
      });
    });
  });
});
