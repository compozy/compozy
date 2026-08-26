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

// The scope hook reads the shell's profile lens and active workspace rather than
// owning a second selector, so both seams are stubbed here.
const shellMocks = vi.hoisted(() => ({
  actingProfile: "default",
  activeWorkspaceId: null as string | null,
  workspaces: [] as { id: string; name: string }[],
}));

vi.mock("@/systems/profiles/hooks/use-profile-read-scope", () => ({
  useProfileReadScope: () => ({ destination: shellMocks.actingProfile }),
  useAggregateDestination: () => null,
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspaceId: shellMocks.activeWorkspaceId,
    workspaces: shellMocks.workspaces,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
}));

import { fetchAgents } from "@/systems/agent/adapters/agent-api";
import { primaryAgentFixture } from "@/systems/agent/mocks";
import { getSettingsSkills, updateSettingsSkills } from "@/systems/settings/adapters/settings-api";
import { settingsSkillSourcesFixture } from "@/systems/settings/mocks";
import { resetSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import type { SettingsMutationResult, SettingsSkillsSection } from "@/systems/settings";
import { fetchWorkspaces } from "@/systems/workspace/adapters/workspace-api";
// The section reads the daemon's coded rejection through the un-mocked error
// module, so the suite throws that exact class rather than a look-alike.
import { SettingsApiError as SourceValidationError } from "@/systems/settings/adapters/settings-api-error";
import { useSettingsSkillsPage } from "../use-settings-skills-page";
import { settingsSkillsDraftLogic } from "../settings-skills-draft-logic";

const skillsEnvelope: SettingsSkillsSection = {
  section: "skills",
  scope: "user",
  available_scopes: ["user"],
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
    sources: ["agents"],
    custom_sources: [],
  },
  sources: settingsSkillSourcesFixture,
  links: [{ label: "skills", path: "/marketplace/skills" }],
};

const skillsMutationFixture: SettingsMutationResult = {
  ...skillsEnvelope,
  active_config_hash: "sha256:skills-live",
  active_generation: 12,
  applied: true,
  apply_record_id: "cfg_apply_skills",
  lifecycle: "live",
  next_action: "none",
  restart_required: false,
  restart_scope: "none",
  warnings: [],
  write_target: "global-config",
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
  shellMocks.actingProfile = "default";
  shellMocks.activeWorkspaceId = null;
  shellMocks.workspaces = [];
  vi.mocked(fetchAgents).mockResolvedValue([]);
  vi.mocked(fetchWorkspaces).mockResolvedValue([]);
  vi.mocked(getSettingsSkills).mockResolvedValue(skillsEnvelope);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSettingsSkillsPage", () => {
  it("Should surface an auxiliary agent catalog failure", async () => {
    vi.mocked(fetchAgents).mockRejectedValue(new Error("Agent catalog unavailable"));
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });

    await waitFor(() => {
      expect(result.current.error).toMatchObject({ message: "Agent catalog unavailable" });
    });
  });

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
      scope: "user",
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
      { scope: "user" }
    );
  });

  it("save policy sends full config with only policy changes and records restart-required label", async () => {
    vi.mocked(updateSettingsSkills).mockResolvedValue({
      section: "skills",
      scope: "user",
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
      { scope: "user" }
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
        scope: "user",
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
        scope: requested.scope ?? "user",
        available_scopes: ["user", "agent"],
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
      sources: ["agents"],
      custom_sources: [],
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
    expect(agentStore.getSnapshot().context.labels).toEqual({
      disabled: null,
      policy: null,
      sources: null,
    });
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
  it("Should key reads by the exact acting profile without an aggregate", async () => {
    shellMocks.actingProfile = "research";
    vi.mocked(getSettingsSkills).mockImplementation(async filter => ({
      ...skillsEnvelope,
      scope: (filter?.scope ?? "user") as SettingsSkillsSection["scope"],
      profile: filter?.profile,
    }));

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });

    await waitFor(() => expect(result.current.envelope).toBeTruthy());
    expect(vi.mocked(getSettingsSkills).mock.calls.at(-1)?.[0]).toEqual({
      scope: "profile",
      profile: "research",
    });
    expect(result.current.personalLabel).toBe("research");
    expect(result.current.actingProfile).toBe("research");
  });

  it("Should preserve the acting profile in a read-only workspace projection", async () => {
    shellMocks.actingProfile = "research";
    shellMocks.activeWorkspaceId = "ws_acme";
    shellMocks.workspaces = [{ id: "ws_acme", name: "acme-api" }];
    vi.mocked(getSettingsSkills).mockImplementation(async filter => ({
      ...skillsEnvelope,
      scope: (filter?.scope ?? "user") as SettingsSkillsSection["scope"],
      profile: filter?.profile,
      workspace_id: filter?.workspace_id,
    }));

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });
    await waitFor(() => expect(result.current.envelope).toBeTruthy());

    act(() => result.current.selectWorkspaceScope());

    await waitFor(() => {
      expect(vi.mocked(getSettingsSkills).mock.calls.at(-1)?.[0]).toEqual({
        scope: "profile",
        profile: "research",
        workspace_id: "ws_acme",
      });
    });
    expect(result.current.isRepositoryProfile).toBe(true);
    expect(result.current.sources.readOnlyReason).toBe("repository-profile");

    act(() => {
      result.current.toggleDisabled("beta");
      result.current.handleSaveDisabled();
      result.current.sources.togglePreset("claude", true);
      result.current.sources.save();
    });

    expect(result.current.isDisabledDirty).toBe(false);
    expect(result.current.sources.isDirty).toBe(false);
    expect(updateSettingsSkills).not.toHaveBeenCalled();
  });

  it("Should keep separate drafts for two profiles under one lens", async () => {
    vi.mocked(getSettingsSkills).mockImplementation(async filter => ({
      ...skillsEnvelope,
      scope: (filter?.scope ?? "user") as SettingsSkillsSection["scope"],
      profile: filter?.profile,
      config: {
        ...skillsEnvelope.config,
        sources: filter?.profile === "research" ? ["agents", "claude"] : ["agents"],
      },
    }));

    const { wrapper, queryClient } = createWrapper();
    const first = renderHook(() => useSettingsSkillsPage(), { wrapper });
    await waitFor(() => expect(first.result.current.draft?.sources).toEqual(["agents"]));
    first.unmount();

    shellMocks.actingProfile = "research";
    const second = renderHook(() => useSettingsSkillsPage(), { wrapper });
    await waitFor(() => expect(second.result.current.draft?.sources).toEqual(["agents", "claude"]));
    // Two cache entries, not one entry answering for both profiles.
    const skillsEntries = queryClient
      .getQueryCache()
      .findAll({ queryKey: ["settings", "section", "skills"] });
    expect(skillsEntries.length).toBeGreaterThan(1);
  });

  it("Should send a workspace override only for the keys the workspace owns", async () => {
    // Invariant: source ownership changes preserve the complete refreshed section.
    // Owning layer: settings skills page hook and draft store.
    // Canonical suite: use-settings-skills-page.test.tsx.
    shellMocks.activeWorkspaceId = "ws_acme";
    shellMocks.workspaces = [{ id: "ws_acme", name: "acme-api" }];
    vi.mocked(getSettingsSkills).mockImplementation(async filter => ({
      ...skillsEnvelope,
      scope: (filter?.scope ?? "user") as SettingsSkillsSection["scope"],
      workspace_id: filter?.workspace_id,
      ...(filter?.scope === "workspace"
        ? { inherits: { sources: true, custom_sources: true } }
        : {}),
    }));
    vi.mocked(updateSettingsSkills).mockResolvedValue({
      ...skillsMutationFixture,
      scope: "workspace",
      workspace_id: "ws_acme",
      write_target: "workspace-config",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });
    await waitFor(() => expect(result.current.envelope).toBeTruthy());

    act(() => result.current.selectWorkspaceScope());
    await waitFor(() => expect(result.current.sources.postures).not.toBeNull());
    expect(result.current.sources.postures).toEqual([
      { key: "sources", inherited: true, armed: false },
      { key: "custom_sources", inherited: true, armed: false },
    ]);

    act(() => result.current.sources.togglePreset("claude", true));
    await waitFor(() => expect(result.current.sources.isDirty).toBe(true));
    expect(
      result.current.sources.groups.presets.find(source => source.slug === "claude")?.enabled
    ).toBe(true);
    act(() => result.current.sources.save());

    await waitFor(() => expect(updateSettingsSkills).toHaveBeenCalled());
    const [body, filter] = vi.mocked(updateSettingsSkills).mock.calls.at(-1) ?? [];
    // custom_sources is still inherited and untouched, so it stays out of the body.
    expect(body).toEqual({ override: { sources: ["agents", "claude"] } });
    expect(filter).toMatchObject({ scope: "workspace", workspace_id: "ws_acme" });
  });

  it("Should return one key to inheritance without touching the other", async () => {
    // Invariant: releasing one source key does not release its sibling key.
    // Owning layer: settings skills page hook and draft store.
    // Canonical suite: use-settings-skills-page.test.tsx.
    shellMocks.activeWorkspaceId = "ws_acme";
    shellMocks.workspaces = [{ id: "ws_acme", name: "acme-api" }];
    vi.mocked(getSettingsSkills).mockImplementation(async filter => ({
      ...skillsEnvelope,
      scope: (filter?.scope ?? "user") as SettingsSkillsSection["scope"],
      workspace_id: filter?.workspace_id,
      ...(filter?.scope === "workspace"
        ? { inherits: { sources: false, custom_sources: true } }
        : {}),
    }));
    vi.mocked(updateSettingsSkills).mockResolvedValue({
      ...skillsMutationFixture,
      scope: "workspace",
      workspace_id: "ws_acme",
      write_target: "workspace-config",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });
    await waitFor(() => expect(result.current.envelope).toBeTruthy());
    act(() => result.current.selectWorkspaceScope());
    await waitFor(() => expect(result.current.sources.postures).not.toBeNull());

    act(() => result.current.sources.useInherited("sources"));

    await waitFor(() => expect(updateSettingsSkills).toHaveBeenCalled());
    expect(vi.mocked(updateSettingsSkills).mock.calls.at(-1)?.[0]).toEqual({
      override: { sources: null },
    });
  });

  it("Should read source policy without writing it at agent scope", async () => {
    vi.mocked(fetchAgents).mockResolvedValue([{ ...primaryAgentFixture, name: "general" }]);
    vi.mocked(getSettingsSkills).mockImplementation(async filter => ({
      ...skillsEnvelope,
      scope: (filter?.scope ?? "user") as SettingsSkillsSection["scope"],
      agent_name: filter?.agent_name,
    }));

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });
    await waitFor(() => expect(result.current.agents).toHaveLength(1));

    act(() => result.current.selectAgentScope());
    await waitFor(() => expect(result.current.sources.readOnly).toBe(true));

    act(() => result.current.sources.togglePreset("claude", true));
    expect(result.current.sources.isDirty).toBe(false);
    expect(updateSettingsSkills).not.toHaveBeenCalled();
  });

  it("Should keep the draft and quote the daemon when a source save is rejected", async () => {
    vi.mocked(updateSettingsSkills).mockRejectedValue(
      new SourceValidationError('unknown skill source preset "cluade"', 400, {
        code: "unknown_skill_source",
        message: 'unknown skill source preset "cluade"',
        valid: ["agents", "claude"],
        suggestion: "claude",
      })
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsSkillsPage(), { wrapper });
    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => result.current.sources.togglePreset("cluade", true));
    await waitFor(() => expect(result.current.sources.isDirty).toBe(true));
    act(() => result.current.sources.save());

    await waitFor(() =>
      expect(result.current.sources.saveError).toBe('unknown skill source preset "cluade"')
    );
    expect(result.current.sources.saveErrorCode).toBe("unknown_skill_source");
    // Nothing was applied, so the operator's edit is still on screen.
    expect(result.current.sources.enabledPresets).toContain("cluade");
    expect(result.current.sources.isDirty).toBe(true);
  });
});
