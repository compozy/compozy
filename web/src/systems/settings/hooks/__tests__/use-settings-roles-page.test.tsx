import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
  useMatchRoute: () => () => false,
}));

vi.mock("@/systems/settings/adapters/settings-api", () => ({
  getSettingsRestartStatus: vi.fn(),
  triggerSettingsRestart: vi.fn(),
  reloadSettings: vi.fn(),
  getRolesStatus: vi.fn(),
  getSettingsRoles: vi.fn(),
  updateSettingsRoles: vi.fn(),
  SettingsApiError: class SettingsApiError extends Error {
    status = 500;
  },
}));

import {
  getRolesStatus,
  getSettingsRoles,
  updateSettingsRoles,
} from "@/systems/settings/adapters/settings-api";
import {
  rolesStatusFixture,
  settingsRolesSectionFixture,
} from "@/systems/settings/mocks/roles-fixtures";
import { resetSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import { settingsKeys } from "@/systems/settings/lib/query-keys";
import { useSettingsRolesPage } from "../use-settings-roles-page";
import { settingsRolesDraftLogic, shouldAdoptRolesBaseline } from "../settings-roles-draft-logic";
import type { SettingsRolesConfig } from "@/systems/settings";

const appliedMutation = {
  section: "roles" as const,
  scope: "global" as const,
  applied: true,
  active_config_hash: "sha256:test",
  active_generation: 1,
  apply_record_id: "cfg_apply_roles",
  lifecycle: "live" as const,
  next_action: "none" as const,
  write_target: "global-config" as const,
};

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { wrapper };
}

beforeEach(() => {
  vi.clearAllMocks();
  resetSettingsRestartStore();
  vi.mocked(getRolesStatus).mockResolvedValue(structuredClone(rolesStatusFixture));
  vi.mocked(getSettingsRoles).mockResolvedValue(structuredClone(settingsRolesSectionFixture));
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSettingsRolesPage", () => {
  it("Should render a warm Query cache without an empty first frame", () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    queryClient.setQueryData(settingsKeys.rolesStatus(), structuredClone(rolesStatusFixture));
    queryClient.setQueryData(
      settingsKeys.section("roles"),
      structuredClone(settingsRolesSectionFixture)
    );
    const warmWrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);
    const renderLengths: number[] = [];

    const { result } = renderHook(
      () => {
        const page = useSettingsRolesPage();
        renderLengths.push(page.roles.length);
        return page;
      },
      { wrapper: warmWrapper }
    );

    expect(renderLengths.length).toBeGreaterThan(0);
    expect(renderLengths.every(length => length === 6)).toBe(true);
    expect(result.current.roles).toHaveLength(6);
  });
  it("Should build the six role view-models in product order once both reads resolve", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRolesPage(), { wrapper });

    await waitFor(() => expect(result.current.roles).toHaveLength(6));
    expect(result.current.roles[0].role).toBe("coordinator");
    expect(result.current.isEmpty).toBe(false);
  });

  it("Should flag an empty projection as a protocol anomaly", async () => {
    vi.mocked(getRolesStatus).mockResolvedValue({ roles: [] });
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRolesPage(), { wrapper });

    await waitFor(() => expect(result.current.isEmpty).toBe(true));
    expect(result.current.roles).toHaveLength(0);
  });

  it("Should reject a partial closed-role projection", async () => {
    vi.mocked(getRolesStatus).mockResolvedValue({ roles: rolesStatusFixture.roles.slice(0, 5) });
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRolesPage(), { wrapper });

    await waitFor(() => expect(result.current.isEmpty).toBe(true));
    expect(result.current.roles).toHaveLength(0);
  });

  it("Should mark dirty on a field edit and reset back to the envelope", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRolesPage(), { wrapper });

    await waitFor(() => expect(result.current.roles).toHaveLength(6));

    act(() => result.current.setRoleField("auto_title", "model", "claude-haiku-4-5"));
    expect(result.current.isDirty).toBe(true);

    const revision = result.current.draftRevision;
    act(() => result.current.handleReset());
    expect(result.current.isDirty).toBe(false);
    expect(result.current.draftRevision).toBe(revision + 1);
  });

  it("Should save the full section and record the applied-immediately label", async () => {
    vi.mocked(updateSettingsRoles).mockResolvedValue(appliedMutation);
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRolesPage(), { wrapper });

    await waitFor(() => expect(result.current.roles).toHaveLength(6));

    // Two events: the edit re-renders the draft, then the save submits it.
    act(() => result.current.setRoleField("auto_title", "model", "claude-haiku-4-5"));
    act(() => result.current.handleSave());

    await waitFor(() =>
      expect(result.current.lastAppliedLabel).toBe("Saved · applied immediately")
    );
    const body = vi.mocked(updateSettingsRoles).mock.calls[0][0];
    expect(body.config.auto_title.model).toBe("claude-haiku-4-5");
  });

  it("Should refetch both reads on retry", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRolesPage(), { wrapper });

    await waitFor(() => expect(result.current.roles).toHaveLength(6));
    expect(getRolesStatus).toHaveBeenCalledTimes(1);
    expect(getSettingsRoles).toHaveBeenCalledTimes(1);

    act(() => result.current.handleRetry());

    await waitFor(() => expect(getRolesStatus).toHaveBeenCalledTimes(2));
    expect(getSettingsRoles).toHaveBeenCalledTimes(2);
  });

  it("Should block save and retain the draft when a fallback entry is invalid", async () => {
    vi.mocked(updateSettingsRoles).mockResolvedValue(appliedMutation);
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRolesPage(), { wrapper });

    await waitFor(() => expect(result.current.roles).toHaveLength(6));

    act(() => result.current.addFallback("dream"));
    expect(result.current.isInvalid).toBe(true);

    act(() => result.current.handleSave());
    expect(updateSettingsRoles).not.toHaveBeenCalled();
    expect(result.current.isDirty).toBe(true);
  });

  it("Should focus an invalid numeric field when it blocks save", async () => {
    vi.mocked(updateSettingsRoles).mockResolvedValue(appliedMutation);
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRolesPage(), { wrapper });
    await waitFor(() => expect(result.current.roles).toHaveLength(6));

    const focus = vi.fn();
    act(() => {
      result.current.registerFieldRef("coordinator.max_children")({
        focus,
      } as unknown as HTMLElement);
      result.current.setNumberFieldValidity("coordinator.max_children")("Enter a value.");
    });
    act(() => result.current.handleSave());

    await waitFor(() => expect(focus).toHaveBeenCalledTimes(1));
    expect(updateSettingsRoles).not.toHaveBeenCalled();
  });

  it("Should preserve an edited roles draft when a newer baseline arrives", () => {
    const baseline = { dream: { agent: "claude" } } as SettingsRolesConfig;
    const edited = { dream: { agent: "codex" } } as SettingsRolesConfig;
    const replacement = { dream: { agent: "hermes" } } as SettingsRolesConfig;
    const store = settingsRolesDraftLogic.createStore({ baseline, lastAppliedLabel: null });

    store.trigger.draftChanged({ draft: edited });

    expect(store.getSnapshot().context.draft).toEqual(edited);
    expect(shouldAdoptRolesBaseline(store.getSnapshot().context, replacement)).toBe(false);
  });
});
