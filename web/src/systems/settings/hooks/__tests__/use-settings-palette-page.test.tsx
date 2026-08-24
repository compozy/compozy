// Suite: Settings Palette page model
// Invariant: the personalization switch reads and writes the scope the operator is actually in,
// asks nothing while workspace truth is unsettled, and never serves one scope's value for another.
// Owning layer: Settings Palette page model. The settings shell suite owns the section list, and
// use-settings-mutations owns apply-record writes — this section echoes itself instead, so neither
// can own this flow.
// Boundary IN: scope derivation, the query key it produces, and the params the adapter receives.
// Boundary OUT: HTTP transport and the daemon's own per-scope storage.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  getSettingsCmdPalette,
  resetCmdPalettePersonalization,
  updateSettingsCmdPalette,
  profile,
  workspace,
} = vi.hoisted(() => ({
  getSettingsCmdPalette: vi.fn(),
  resetCmdPalettePersonalization: vi.fn(),
  updateSettingsCmdPalette: vi.fn(),
  profile: { destination: "default" },
  workspace: {
    scope: "global" as "global" | "workspace",
    activeWorkspaceId: null as string | null,
    hasHydrated: true,
    isLoading: false,
    /** The resolver's own signal: the catalog can be loaded while `$HOME` is not. */
    pending: false,
    error: null as Error | null,
    refetch: vi.fn(),
    runtimeWorkspaceId: "workspace:home" as string | null,
    activeWorkspace: undefined as { name: string } | undefined,
  },
}));

vi.mock("../../adapters/settings-sections-api", () => ({
  getSettingsCmdPalette,
  updateSettingsCmdPalette,
}));
vi.mock("@/systems/workspace", () => ({ useActiveWorkspace: () => workspace }));
// Personalization is per profile; which profile is the shell's business, not
// this page's, so the acting one is stubbed at its own seam.
vi.mock("@/systems/profiles", () => ({
  useProfileReadScope: () => ({
    destination: profile.destination,
    key: profile.destination,
    aggregate: false,
  }),
}));
vi.mock("@/systems/os", () => ({
  cmdPaletteKeys: { all: ["cmd-palette"] },
  resetCmdPalettePersonalization,
}));
vi.mock("../use-settings-page", () => ({
  useSettingsPage: () => ({ restart: { isVisible: false } }),
}));

import { settingsKeys } from "../../lib/query-keys";
import { useSettingsPalettePage } from "../use-settings-palette-page";

function section(personalization: boolean, scope: "user" | "profile" | "workspace" = "user") {
  return {
    section: "cmd-palette",
    scope,
    available_scopes: ["user", "profile", "workspace"],
    fallback_agent_enabled: true,
    personalization,
  };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { client, ...renderHook(() => useSettingsPalettePage(), { wrapper }) };
}

describe("useSettingsPalettePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    workspace.scope = "global";
    profile.destination = "default";
    workspace.activeWorkspaceId = null;
    workspace.hasHydrated = true;
    workspace.isLoading = false;
    workspace.pending = false;
    workspace.error = null;
    workspace.refetch.mockReset();
    workspace.runtimeWorkspaceId = "workspace:home";
    workspace.activeWorkspace = undefined;
    getSettingsCmdPalette.mockResolvedValue(section(true));
    resetCmdPalettePersonalization.mockResolvedValue(undefined);
    updateSettingsCmdPalette.mockResolvedValue(section(false));
  });

  it.each([
    { label: "the store has not hydrated", mutate: () => (workspace.hasHydrated = false) },
    { label: "the workspace catalog is loading", mutate: () => (workspace.isLoading = true) },
    {
      // The catalog can be loaded while `$HOME` is still unknown, and the
      // resolver settles nothing until both land — so a requested workspace
      // still reads as user scope here.
      label: "the resolver is still pending on $HOME",
      mutate: () => {
        workspace.pending = true;
        workspace.scope = "workspace";
      },
    },
  ])("Should ask nothing while $label", async ({ mutate }) => {
    mutate();
    const { result } = renderPage();

    await waitFor(() => expect(result.current.isLoading).toBe(true));
    expect(getSettingsCmdPalette).not.toHaveBeenCalled();
  });

  it("Should read and write user scope when no workspace is active", async () => {
    const { result } = renderPage();

    await waitFor(() => expect(result.current.section?.personalization).toBe(true));
    expect(getSettingsCmdPalette).toHaveBeenCalledWith({ scope: "user" }, expect.anything());

    act(() => result.current.setPersonalization(false));

    await waitFor(() => expect(updateSettingsCmdPalette).toHaveBeenCalledTimes(1));
    expect(updateSettingsCmdPalette).toHaveBeenCalledWith(
      { personalization: false },
      { scope: "user" }
    );
  });

  it("Should write the fallback toggle without replacing personalization [IT-015]", async () => {
    const { result } = renderPage();

    await waitFor(() => expect(result.current.section?.fallback_agent_enabled).toBe(true));
    act(() => result.current.setFallbackAgentEnabled(false));

    await waitFor(() => expect(updateSettingsCmdPalette).toHaveBeenCalledTimes(1));
    expect(updateSettingsCmdPalette).toHaveBeenCalledWith(
      { fallback_agent_enabled: false },
      { scope: "user" }
    );
  });

  it("Should read and write the active profile before workspace scope", async () => {
    profile.destination = "marketing";
    workspace.scope = "workspace";
    workspace.activeWorkspaceId = "workspace:alpha";
    const { result } = renderPage();

    await waitFor(() => expect(result.current.section?.personalization).toBe(true));
    expect(getSettingsCmdPalette).toHaveBeenCalledWith(
      { scope: "profile", profile: "marketing" },
      expect.anything()
    );

    act(() => result.current.setPersonalization(false));

    await waitFor(() => expect(updateSettingsCmdPalette).toHaveBeenCalledTimes(1));
    expect(updateSettingsCmdPalette).toHaveBeenCalledWith(
      { personalization: false },
      { scope: "profile", profile: "marketing" }
    );
  });

  it("Should reset personalization only after the explicit action [UT-151]", async () => {
    const { result } = renderPage();

    await waitFor(() => expect(result.current.section).not.toBeNull());
    expect(resetCmdPalettePersonalization).not.toHaveBeenCalled();

    await act(async () => result.current.resetPersonalization());

    // A reset acts as one profile — the aggregate keeps its own history and is
    // refused server-side, so the acting profile rides the call.
    expect(resetCmdPalettePersonalization).toHaveBeenCalledWith("workspace:home", "default");
  });

  it("Should disable profile reset until the runtime workspace is ready", async () => {
    profile.destination = "marketing";
    workspace.runtimeWorkspaceId = null;
    const { result } = renderPage();

    await waitFor(() => expect(result.current.section).not.toBeNull());
    expect(result.current.canResetPersonalization).toBe(false);
    await act(async () => result.current.resetPersonalization());
    expect(resetCmdPalettePersonalization).not.toHaveBeenCalled();
  });

  it("Should keep reset pending state and errors on the profile that started them", async () => {
    let rejectReset: ((error: Error) => void) | undefined;
    resetCmdPalettePersonalization.mockImplementation(
      () =>
        new Promise((_resolve, reject) => {
          rejectReset = reject;
        })
    );
    profile.destination = "marketing";
    const { rerender, result } = renderPage();
    await waitFor(() => expect(result.current.section).not.toBeNull());

    let request: Promise<void> | undefined;
    act(() => {
      request = result.current.resetPersonalization();
    });
    await waitFor(() => expect(result.current.isResetting).toBe(true));

    profile.destination = "sales";
    rerender();
    expect(result.current.isResetting).toBe(false);
    expect(result.current.resetError).toBeNull();

    await act(async () => {
      rejectReset?.(new Error("marketing reset failed"));
      await request?.catch(() => undefined);
    });
    expect(result.current.resetError).toBeNull();
  });

  it("Should read and write the active workspace when that is the effective scope", async () => {
    workspace.scope = "workspace";
    workspace.activeWorkspaceId = "workspace:alpha";
    workspace.runtimeWorkspaceId = "workspace:alpha";
    workspace.activeWorkspace = { name: "Alpha" };
    const { result } = renderPage();

    await waitFor(() => expect(result.current.section?.personalization).toBe(true));
    expect(getSettingsCmdPalette).toHaveBeenCalledWith(
      { scope: "workspace", workspace_id: "workspace:alpha" },
      expect.anything()
    );

    act(() => result.current.setPersonalization(false));

    await waitFor(() => expect(updateSettingsCmdPalette).toHaveBeenCalledTimes(1));
    expect(updateSettingsCmdPalette).toHaveBeenCalledWith(
      { personalization: false },
      { scope: "workspace", workspace_id: "workspace:alpha" }
    );
  });

  it("Should echo the write into that scope's cache entry alone", async () => {
    workspace.scope = "workspace";
    workspace.activeWorkspaceId = "workspace:alpha";
    const { client, result } = renderPage();

    await waitFor(() => expect(result.current.section).not.toBeNull());
    act(() => result.current.setPersonalization(false));
    await waitFor(() => expect(result.current.section?.personalization).toBe(false));

    const scoped = settingsKeys.cmdPaletteSection({
      scope: "workspace",
      workspace_id: "workspace:alpha",
    });
    expect(client.getQueryData(scoped)).toMatchObject({ personalization: false });
    // The scope the operator was not in keeps whatever it had — here, nothing.
    expect(client.getQueryData(settingsKeys.cmdPaletteSection({ scope: "user" }))).toBeUndefined();
  });

  it("Should file a late write under the scope it was made in [regression]", async () => {
    // A write issued in workspace A can land after the operator moved to B.
    // The scope has to travel with the write: reading it from the current
    // render would file A's answer under B and paint A's pending value there.
    // Both scopes read as on, so the only way a row can read off is a leak:
    // either A's pending overlay painted on B, or A's echo filed under B.
    getSettingsCmdPalette.mockResolvedValue(section(true, "workspace"));
    let resolveWrite: ((value: unknown) => void) | undefined;
    updateSettingsCmdPalette.mockImplementation(
      () =>
        new Promise(resolve => {
          resolveWrite = resolve;
        })
    );
    workspace.scope = "workspace";
    workspace.activeWorkspaceId = "workspace:alpha";
    const { client, rerender, result } = renderPage();
    await waitFor(() => expect(result.current.section?.personalization).toBe(true));

    act(() => result.current.setPersonalization(false));
    // Still in A: the pending value is A's to show.
    await waitFor(() => expect(result.current.section?.personalization).toBe(false));

    workspace.activeWorkspaceId = "workspace:beta";
    rerender();

    // B's own value, never A's in-flight one — the write is still open.
    await waitFor(() => expect(result.current.section?.personalization).toBe(true));
    expect(result.current.isSaving).toBe(true);

    await act(async () => {
      resolveWrite?.(section(false, "workspace"));
      await Promise.resolve();
    });

    const alpha = settingsKeys.cmdPaletteSection({
      scope: "workspace",
      workspace_id: "workspace:alpha",
    });
    const beta = settingsKeys.cmdPaletteSection({
      scope: "workspace",
      workspace_id: "workspace:beta",
    });
    await waitFor(() =>
      expect(client.getQueryData(alpha)).toMatchObject({ personalization: false })
    );
    // B was never written to by A's answer, and still shows what B returned.
    expect(client.getQueryData(beta)).toMatchObject({ personalization: true });
    expect(result.current.section?.personalization).toBe(true);
  });

  it("Should never serve one scope's value for another [regression]", async () => {
    getSettingsCmdPalette.mockImplementation((filter: { workspace_id?: string }) =>
      Promise.resolve(section(filter.workspace_id !== "workspace:beta"))
    );
    workspace.scope = "workspace";
    workspace.activeWorkspaceId = "workspace:alpha";
    const { rerender, result } = renderPage();

    await waitFor(() => expect(result.current.section?.personalization).toBe(true));

    workspace.activeWorkspaceId = "workspace:beta";
    rerender();
    await waitFor(() => expect(result.current.section?.personalization).toBe(false));

    workspace.scope = "global";
    workspace.activeWorkspaceId = null;
    rerender();
    await waitFor(() => expect(result.current.section?.personalization).toBe(true));
    expect(getSettingsCmdPalette).toHaveBeenCalledTimes(3);
  });

  it("Should surface a workspace catalog failure instead of spinning", async () => {
    workspace.pending = true;
    workspace.error = new Error("workspace catalog failed");
    const { result } = renderPage();

    await waitFor(() => expect(result.current.error?.message).toBe("workspace catalog failed"));
    expect(result.current.isLoading).toBe(false);
    expect(getSettingsCmdPalette).not.toHaveBeenCalled();

    result.current.handleRetry();
    expect(workspace.refetch).toHaveBeenCalled();
  });
});
