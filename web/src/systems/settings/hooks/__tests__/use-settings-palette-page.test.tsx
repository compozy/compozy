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

const { getSettingsCmdPalette, updateSettingsCmdPalette, workspace } = vi.hoisted(() => ({
  getSettingsCmdPalette: vi.fn(),
  updateSettingsCmdPalette: vi.fn(),
  workspace: {
    scope: "global" as "global" | "workspace",
    activeWorkspaceId: null as string | null,
    hasHydrated: true,
    isLoading: false,
    /** The resolver's own signal: the catalog can be loaded while `$HOME` is not. */
    pending: false,
  },
}));

vi.mock("../../adapters/settings-sections-api", () => ({
  getSettingsCmdPalette,
  updateSettingsCmdPalette,
}));
vi.mock("@/systems/workspace", () => ({ useActiveWorkspace: () => workspace }));
vi.mock("@/systems/os", () => ({ cmdPaletteKeys: { all: ["cmd-palette"] } }));
vi.mock("../use-settings-page", () => ({
  useSettingsPage: () => ({ restart: { isVisible: false } }),
}));

import { settingsKeys } from "../../lib/query-keys";
import { useSettingsPalettePage } from "../use-settings-palette-page";

function section(personalization: boolean, scope: "global" | "workspace" = "global") {
  return {
    section: "cmd-palette",
    scope,
    available_scopes: ["global", "workspace"],
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
    workspace.activeWorkspaceId = null;
    workspace.hasHydrated = true;
    workspace.isLoading = false;
    workspace.pending = false;
    getSettingsCmdPalette.mockResolvedValue(section(true));
    updateSettingsCmdPalette.mockResolvedValue(section(false));
  });

  it.each([
    { label: "the store has not hydrated", mutate: () => (workspace.hasHydrated = false) },
    { label: "the workspace catalog is loading", mutate: () => (workspace.isLoading = true) },
    {
      // The catalog can be loaded while `$HOME` is still unknown, and the
      // resolver settles nothing until both land — so a requested workspace
      // still reads as global here.
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

  it("Should read and write global scope when no workspace is active", async () => {
    const { result } = renderPage();

    await waitFor(() => expect(result.current.section?.personalization).toBe(true));
    expect(getSettingsCmdPalette).toHaveBeenCalledWith({ scope: "global" }, expect.anything());

    act(() => result.current.setPersonalization(false));

    await waitFor(() => expect(updateSettingsCmdPalette).toHaveBeenCalledTimes(1));
    expect(updateSettingsCmdPalette).toHaveBeenCalledWith(
      { personalization: false },
      { scope: "global" }
    );
  });

  it("Should read and write the active workspace when that is the effective scope", async () => {
    workspace.scope = "workspace";
    workspace.activeWorkspaceId = "workspace:alpha";
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
    expect(
      client.getQueryData(settingsKeys.cmdPaletteSection({ scope: "global" }))
    ).toBeUndefined();
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
});
