// Suite: command palette API adapter
// Invariant: a 200 view-session open without stream_token, view_session, or
// first_frame is a malformed_response, not a usable session; typed daemon
// errors keep code/reason/fields; query and body stay scoped to the workspace plus profile lens.
// Owning layer: unit. Canonical suite for adapter envelope validation.
// Boundary IN: open/get/list/invoke adapters.
// Boundary OUT: SSE and the program-view hook.

import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  CmdPaletteApiError,
  cmdPaletteViewSessionStreamURL,
  getCmdPaletteView,
  invokeCmdPaletteCommand,
  listCmdPaletteCommands,
  openCmdPaletteViewSession,
} from "../cmd-palette-api";

vi.mock("@/lib/api-client", () => ({
  apiClient: {
    GET: vi.fn(),
    POST: vi.fn(),
    PUT: vi.fn(),
    DELETE: vi.fn(),
  },
  apiRequestFailed: (response: Response, error: unknown) => !response.ok || error !== undefined,
  apiErrorMessage: () => undefined,
}));

import { apiClient } from "@/lib/api-client";
import { CMD_PALETTE_AGGREGATE_LENS_KEY } from "../../lib/cmd-palette-query-keys";

describe("command palette API adapter", () => {
  beforeEach(() => {
    vi.mocked(apiClient.GET).mockReset();
    vi.mocked(apiClient.POST).mockReset();
  });

  it("Should reject an incomplete view-session open envelope [RD0068]", async () => {
    vi.mocked(apiClient.POST).mockResolvedValue({
      data: {},
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);

    await expect(
      openCmdPaletteViewSession("ws", "marketing", "ext.notes.browser", "token")
    ).rejects.toMatchObject({
      kind: "malformed_response",
      name: CmdPaletteApiError.name,
    });
  });

  it("Should reject a view GET that omits the revision fence [RA0252]", async () => {
    vi.mocked(apiClient.GET).mockResolvedValue({
      data: { view_id: "ext.notes.browser", title: "Notes", kind: "list", payload: { view: "v1" } },
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);

    await expect(getCmdPaletteView("ws", "default", "ext.notes.browser")).rejects.toMatchObject({
      kind: "malformed_response",
    });
  });

  it("Should reject a catalog that omits catalog_revision [RA0252]", async () => {
    vi.mocked(apiClient.GET).mockResolvedValue({
      data: { commands: [], sources: [] },
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);

    await expect(listCmdPaletteCommands("ws", "default", "client-1")).rejects.toMatchObject({
      kind: "malformed_response",
    });
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/api/cmd-palette/commands",
      expect.objectContaining({
        // The lens is never omitted: an absent selector resolves `default`
        // server-side, which would silently answer for the wrong profile.
        params: { query: { workspace: "ws", client: "client-1", profile: "default" } },
      })
    );
  });

  it("Should encode the aggregate profile lens in the catalog request", async () => {
    vi.mocked(apiClient.GET).mockResolvedValue({
      data: { commands: [], sources: [], catalog_revision: "rev_1" },
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);

    await listCmdPaletteCommands("ws", CMD_PALETTE_AGGREGATE_LENS_KEY, null);

    expect(apiClient.GET).toHaveBeenCalledWith(
      "/api/cmd-palette/commands",
      expect.objectContaining({
        params: { query: { workspace: "ws", all_profiles: true } },
      })
    );
  });

  it("Should keep daemon error code, reason, and fields [RA0252]", async () => {
    vi.mocked(apiClient.POST).mockResolvedValue({
      data: undefined,
      error: {
        error: "no_attached_shell",
        message: "Attach a shell first.",
        reason: "requires an attached shell",
        fields: { client: "required" },
      },
      response: new Response(null, { status: 409 }),
    } as never);

    await expect(
      invokeCmdPaletteCommand({ workspaceId: "ws", profile: "marketing", commandId: "notes.open" })
    ).rejects.toMatchObject({
      kind: "daemon",
      status: 409,
      code: "no_attached_shell",
      reason: "requires an attached shell",
      fields: { client: "required" },
    });
    expect(apiClient.POST).toHaveBeenCalledWith(
      "/api/cmd-palette/commands/{id}/invoke",
      expect.objectContaining({
        params: {
          path: { id: "notes.open" },
          header: { "X-Compozy-Client-Token": "" },
          query: { profile: "marketing" },
        },
        body: { workspace: "ws", args: {} },
      })
    );
  });

  it("Should publish client id and attachment token together on invoke [RD0082]", async () => {
    vi.mocked(apiClient.POST).mockResolvedValue({
      data: { status: "ok" },
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);

    await invokeCmdPaletteCommand({
      workspaceId: "ws",
      profile: "marketing",
      commandId: "notes.open",
      clientId: "client-1",
      attachmentToken: "tok",
    });
    expect(apiClient.POST).toHaveBeenCalledWith(
      "/api/cmd-palette/commands/{id}/invoke",
      expect.objectContaining({
        params: {
          path: { id: "notes.open" },
          header: { "X-Compozy-Client-Token": "tok" },
          query: { profile: "marketing" },
        },
        body: { workspace: "ws", args: {}, client: "client-1" },
      })
    );
  });

  it("Should send an empty invoke token header when no attachment exists [RD0082]", async () => {
    vi.mocked(apiClient.POST).mockResolvedValue({
      data: { status: "ok" },
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);

    await invokeCmdPaletteCommand({
      workspaceId: "ws",
      profile: "marketing",
      commandId: "notes.open",
    });
    expect(apiClient.POST).toHaveBeenCalledWith(
      "/api/cmd-palette/commands/{id}/invoke",
      expect.objectContaining({
        params: {
          path: { id: "notes.open" },
          header: { "X-Compozy-Client-Token": "" },
          query: { profile: "marketing" },
        },
        body: { workspace: "ws", args: {} },
      })
    );
  });

  it("Should encode the open workspace and attachment token [RA0252]", async () => {
    vi.mocked(apiClient.POST).mockResolvedValue({
      data: {
        stream_token: "st",
        view_session: "vs",
        first_frame: {
          revision: "vr_1",
          view_session: "vs",
          generation: 1,
          handlers: [],
        },
      },
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);

    await openCmdPaletteViewSession("ws", "marketing", "ext.notes.browser", "tok");
    expect(apiClient.POST).toHaveBeenCalledWith(
      "/api/cmd-palette/views/{id}/open",
      expect.objectContaining({
        params: {
          path: { id: "ext.notes.browser" },
          header: { "X-Compozy-Client-Token": "tok" },
          query: { profile: "marketing" },
        },
        body: { workspace: "ws", args: {} },
      })
    );
  });

  it("Should encode the view-session stream URL [RA0252]", () => {
    expect(cmdPaletteViewSessionStreamURL("vs/a", "t k")).toBe(
      "/api/cmd-palette/view-sessions/vs%2Fa/stream?token=t%20k"
    );
  });

  it("Should bind a view session to the acting profile rather than resolving default", async () => {
    vi.mocked(apiClient.POST).mockResolvedValue({
      data: {
        view_session: "vs_1",
        stream_token: "st_1",
        profile_lens: { profile_lens_id: "01J9MARKETING00000000000000", profile_name: "marketing" },
        first_frame: {
          view_session: "vs_1",
          revision: "vr_1",
          seq: 1,
          effects: [],
          generation: 1,
          handlers: [],
        },
      },
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);

    await openCmdPaletteViewSession("ws", "marketing", "ext.notes.browser", "tok");
    // A programmable view runs extension code under an owner; opening it without
    // a selector would bind the session to `default` from any profile.
    expect(apiClient.POST).toHaveBeenLastCalledWith(
      "/api/cmd-palette/views/{id}/open",
      expect.objectContaining({
        params: expect.objectContaining({ query: { profile: "marketing" } }),
      })
    );
  });
});
