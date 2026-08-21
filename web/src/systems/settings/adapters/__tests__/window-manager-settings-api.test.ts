// Suite: window-manager settings section adapter
// Invariant: every read and write is scoped to one trimmed workspace/client, a
// refused mutation keeps the daemon's conflict names, and a 200 that is not a
// window-manager section is not treated as a usable section.
// Owning layer: unit. Canonical suite for this adapter's envelope — layouts
// snapshot/CAS stays on window-manager-layouts-api.test.ts; general settings
// fetch stays on settings-api.test.ts; the shortcut table spies the write.
// Boundary IN: fetchWindowManagerSettings / updateWindowManagerBindings.
// Boundary OUT: React Query cache and the settings shortcut surfaces.
// Not duplicating: cmd-palette-api.test.ts (palette view/catalog transport).

import { afterEach, describe, expect, it, vi } from "vitest";
import { ZodError } from "zod";

import {
  fetchWindowManagerSettings,
  parseSettingsWindowManagerSection,
  updateWindowManagerBindings,
  WindowManagerSettingsError,
  type WindowManagerSettingsWire,
} from "@/systems/os";

import { settingsWindowManagerSectionFixture } from "../../mocks/window-manager-fixtures";

vi.mock("@/lib/api-client", async importOriginal => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();
  return {
    ...actual,
    apiClient: {
      GET: vi.fn(),
      PATCH: vi.fn(),
    },
  };
});

import { apiClient } from "@/lib/api-client";

function sectionWire(): WindowManagerSettingsWire {
  return structuredClone(settingsWindowManagerSectionFixture) as WindowManagerSettingsWire;
}

describe("window-manager settings API adapter", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("Should reject a 200 that omits the required command list [RA0252]", async () => {
    const wire = sectionWire();
    delete (wire as { commands?: unknown }).commands;
    vi.mocked(apiClient.GET).mockResolvedValue({
      data: wire,
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);

    await expect(
      fetchWindowManagerSettings({ workspaceId: "workspace:alpha" })
    ).rejects.toBeInstanceOf(ZodError);
  });

  it("Should reject an empty 200 instead of inventing a section [RA0252]", async () => {
    vi.mocked(apiClient.GET).mockResolvedValue({
      data: undefined,
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);

    await expect(fetchWindowManagerSettings({ workspaceId: "workspace:alpha" })).rejects.toThrow(
      /Unable to load window-management settings\.: empty response \(200\)/
    );
  });

  it("Should keep daemon conflict code, owner, chord, and alias [RA0252]", async () => {
    vi.mocked(apiClient.PATCH).mockResolvedValue({
      data: undefined,
      error: {
        error: "shortcut_conflict",
        message: "⌘N is already used by New session",
        owner: "session.new",
        chord: "meta+KeyN",
        alias: "ns",
      },
      response: new Response(null, { status: 409 }),
    } as never);

    await expect(
      updateWindowManagerBindings({
        workspaceId: "workspace:alpha",
        shortcuts: { "ext.notes.capture": ["meta+KeyN"] },
      })
    ).rejects.toMatchObject({
      name: WindowManagerSettingsError.name,
      status: 409,
      code: "shortcut_conflict",
      owner: "session.new",
      chord: "meta+KeyN",
      alias: "ns",
      message: "⌘N is already used by New session",
    });
  });

  it("Should drop blank conflict names and unknown mutation codes [RA0252]", async () => {
    vi.mocked(apiClient.PATCH).mockResolvedValue({
      data: undefined,
      error: {
        error: "not_a_settings_code",
        owner: "   ",
        chord: "",
        alias: "  ",
      },
      response: new Response(null, { status: 500 }),
    } as never);

    await expect(
      updateWindowManagerBindings({ workspaceId: "workspace:alpha", aliases: {} })
    ).rejects.toMatchObject({
      name: WindowManagerSettingsError.name,
      status: 500,
      code: null,
      owner: null,
      chord: null,
      alias: null,
      message: "Unable to save the keyboard shortcut.",
    });
  });

  it("Should encode trimmed workspace and client on the query [RA0252]", async () => {
    const wire = sectionWire();
    vi.mocked(apiClient.GET).mockResolvedValue({
      data: wire,
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);

    const section = await fetchWindowManagerSettings({
      workspaceId: "  workspace:alpha  ",
      clientId: "  shell-1  ",
    });

    expect(apiClient.GET).toHaveBeenCalledWith(
      "/api/settings/window-manager",
      expect.objectContaining({
        params: {
          query: {
            scope: "workspace",
            workspace_id: "workspace:alpha",
            client_id: "shell-1",
          },
        },
      })
    );
    expect(section.workspaceId).toBe(wire.workspace_id);
    expect(section.commands).toHaveLength(wire.commands.length);
    expect(section).toEqual(parseSettingsWindowManagerSection(wire));
  });

  it("Should encode global scope when the workspace token is empty [RA0252]", async () => {
    vi.mocked(apiClient.GET).mockResolvedValue({
      data: sectionWire(),
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);

    await fetchWindowManagerSettings({ workspaceId: "   " });

    expect(apiClient.GET).toHaveBeenLastCalledWith(
      "/api/settings/window-manager",
      expect.objectContaining({
        params: { query: { scope: "global" } },
      })
    );
  });

  it("Should send complete maps and keep overwrite off the wire until asked [RA0252]", async () => {
    const wire = sectionWire();
    vi.mocked(apiClient.PATCH).mockResolvedValue({
      data: wire,
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);

    await updateWindowManagerBindings({
      workspaceId: "workspace:alpha",
      shortcuts: { "window.close": ["meta+KeyW"] },
      globalShortcuts: { "palette.summon.global": "meta+shift+KeyK" },
      aliases: { "session.new": "ns" },
    });

    expect(apiClient.PATCH).toHaveBeenCalledWith(
      "/api/settings/window-manager",
      expect.objectContaining({
        params: { query: { scope: "workspace", workspace_id: "workspace:alpha" } },
        body: {
          shortcuts: { "window.close": ["meta+KeyW"] },
          global_shortcuts: { "palette.summon.global": "meta+shift+KeyK" },
          aliases: { "session.new": "ns" },
        },
      })
    );

    await updateWindowManagerBindings({
      workspaceId: "workspace:alpha",
      shortcuts: { "window.close": ["meta+KeyW"] },
      overwrite: true,
    });

    expect(apiClient.PATCH).toHaveBeenLastCalledWith(
      "/api/settings/window-manager",
      expect.objectContaining({
        body: expect.objectContaining({ overwrite: true }),
      })
    );
  });
});
