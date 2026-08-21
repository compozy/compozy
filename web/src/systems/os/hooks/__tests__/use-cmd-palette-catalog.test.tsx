// Suite: cmd-palette catalog hydration
// Invariant: the last-known structural catalog renders immediately, the fetched
// catalog replaces it wholesale at one revision (never a partial merge), a
// successful fetch persists structure only, and an unreachable daemon keeps the
// cached structure while reporting itself unreachable.
// Boundary IN: the hydration hook's cache seeding, replacement and persistence.
// Boundary OUT: availability evaluation, invalidation streaming, and dispatch.
import "fake-indexeddb/auto";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import * as api from "../../adapters/cmd-palette-api";
import { CmdPaletteApiError } from "../../adapters/cmd-palette-api";
import {
  dropCatalogRecord,
  readCatalogRecord,
  writeCatalogRecord,
} from "../../lib/cmd-palette-catalog-cache";
import { toCatalogRecord } from "../../lib/cmd-palette-catalog-record";
import type { CmdPaletteCatalogResponse } from "../../lib/cmd-palette-types";
import { useCmdPaletteCatalog } from "../use-cmd-palette-catalog";

const WORKSPACE = "ws-hq";
const OTHER_WORKSPACE = "ws-branch";
const CLIENT = "client-1";

function catalog(
  revision: string,
  titles: readonly string[],
  profileName = "default"
): CmdPaletteCatalogResponse {
  return {
    profile_lens: { profile_lens_id: "00000000000000000000000000", profile_name: profileName },
    catalog_revision: revision,
    context_revision: "9",
    commands: titles.map(title => ({
      id: title.toLowerCase().replaceAll(" ", "."),
      title,
      section: "Shell",
      icon: "command",
      source: "core",
      available: true,
      bindings: [],
      alias: null,
      destructive: false,
      availability_exempt: false,
      arguments: [],
      action: { kind: "client_op", op: title.toLowerCase().replaceAll(" ", ".") },
      execution: { retry_safe: true, single_flight: false },
    })),
    sources: [{ source: "core", status: "healthy" }],
  };
}

function harness(options: { retry?: number; retryDelay?: number } = {}) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: options.retry ?? false,
        ...(options.retryDelay === undefined ? {} : { retryDelay: options.retryDelay }),
      },
      mutations: { retry: false },
    },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return {
    ...renderHook(() => useCmdPaletteCatalog({ workspaceId: WORKSPACE, clientId: CLIENT }), {
      wrapper,
    }),
    queryClient,
  };
}

describe("useCmdPaletteCatalog (UT-095)", () => {
  beforeEach(async () => {
    await Promise.all([
      dropCatalogRecord(WORKSPACE, "default"),
      dropCatalogRecord(OTHER_WORKSPACE, "default"),
    ]);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should render the last-known catalog before the daemon answers, then reconcile to the fetched revision", async () => {
    await writeCatalogRecord(
      toCatalogRecord(WORKSPACE, "default", catalog("sha256:cached", ["Cached row"]))
    );
    let release: (value: CmdPaletteCatalogResponse) => void = () => {};
    vi.spyOn(api, "listCmdPaletteCommands").mockReturnValue(
      new Promise<CmdPaletteCatalogResponse>(resolve => {
        release = resolve;
      })
    );

    const { result } = harness();
    await waitFor(() => expect(result.current.catalog?.catalogRevision).toBe("sha256:cached"));
    // Stale, but rendered: the palette never waits on the daemon to open.
    expect(result.current.stale).toBe(true);
    expect(result.current.daemonReachable).toBe(false);
    expect(result.current.catalog?.commands.map(command => command.title)).toEqual(["Cached row"]);

    release(catalog("sha256:fresh", ["Fresh row", "Second row"]));
    await waitFor(() => expect(result.current.catalog?.catalogRevision).toBe("sha256:fresh"));
    expect(result.current.stale).toBe(false);
    expect(result.current.daemonReachable).toBe(true);
    // Replaced wholesale — the stale row is gone rather than merged alongside.
    expect(result.current.catalog?.commands.map(command => command.title)).toEqual([
      "Fresh row",
      "Second row",
    ]);
  });

  it("Should persist structure only after a successful fetch", async () => {
    vi.spyOn(api, "listCmdPaletteCommands").mockResolvedValue(
      catalog("sha256:persisted", ["Persisted row"])
    );
    const { result } = harness();
    await waitFor(() => expect(result.current.daemonReachable).toBe(true));
    await waitFor(async () =>
      expect((await readCatalogRecord(WORKSPACE, "default"))?.catalogRevision).toBe(
        "sha256:persisted"
      )
    );
    const stored = await readCatalogRecord(WORKSPACE, "default");
    for (const command of stored?.commands ?? []) {
      expect(command).not.toHaveProperty("available");
      expect(command).not.toHaveProperty("reason");
    }
  });

  it("Should carry a catalog only across client attachment in the same workspace", async () => {
    let releaseClientCatalog: (value: CmdPaletteCatalogResponse) => void = () => {};
    vi.spyOn(api, "listCmdPaletteCommands").mockImplementation(
      (workspace, _profileKey, clientId) => {
        if (workspace === WORKSPACE && clientId === null) {
          return Promise.resolve(catalog("sha256:workspace", ["Settings"]));
        }
        return new Promise<CmdPaletteCatalogResponse>(resolve => {
          releaseClientCatalog = resolve;
        });
      }
    );
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result, rerender } = renderHook(
      ({ workspaceId, clientId }: { workspaceId: string; clientId: string | null }) =>
        useCmdPaletteCatalog({ workspaceId, clientId }),
      {
        initialProps: { workspaceId: WORKSPACE, clientId: null as string | null },
        wrapper,
      }
    );
    await waitFor(() => expect(result.current.catalog?.catalogRevision).toBe("sha256:workspace"));

    rerender({ workspaceId: WORKSPACE, clientId: CLIENT });
    expect(result.current.catalog?.commands.map(command => command.title)).toEqual(["Settings"]);
    expect(result.current.daemonReachable).toBe(true);

    rerender({ workspaceId: OTHER_WORKSPACE, clientId: CLIENT });
    expect(result.current.catalog).toBeNull();
    expect(result.current.daemonReachable).toBe(false);

    releaseClientCatalog(catalog("sha256:client", ["Settings", "Close window"]));
    await waitFor(() => expect(result.current.catalog?.catalogRevision).toBe("sha256:client"));
  });

  it("Should keep the cached catalog and report the daemon unreachable when the fetch fails", async () => {
    await writeCatalogRecord(
      toCatalogRecord(WORKSPACE, "default", catalog("sha256:cached", ["Cached row"]))
    );
    vi.spyOn(api, "listCmdPaletteCommands").mockRejectedValue(
      new CmdPaletteApiError("Command palette unavailable", 503, "daemon")
    );
    const { result } = harness();
    await waitFor(() => expect(result.current.catalog?.catalogRevision).toBe("sha256:cached"));
    expect(result.current.daemonReachable).toBe(false);
    expect(result.current.stale).toBe(true);
  });

  it("Should stop reporting the daemon reachable on the first failed refetch before its retry", async () => {
    const list = vi
      .spyOn(api, "listCmdPaletteCommands")
      .mockResolvedValue(catalog("sha256:live", ["Live row"]));
    const { result, queryClient, unmount } = harness({ retry: 1, retryDelay: 60_000 });
    await waitFor(() => expect(result.current.daemonReachable).toBe(true));

    // The daemon goes away; its last answer is still the best structure to
    // render, but nothing may claim it is reachable.
    list.mockRejectedValue(new CmdPaletteApiError("Command palette unavailable", 503, "daemon"));
    void queryClient.refetchQueries();

    await waitFor(() => expect(result.current.daemonReachable).toBe(false));
    expect(result.current.stale).toBe(true);
    expect(result.current.catalog?.commands.map(command => command.title)).toEqual(["Live row"]);
    unmount();
    queryClient.clear();
  });

  it("Should report a cold open with no cache and no daemon", async () => {
    vi.spyOn(api, "listCmdPaletteCommands").mockRejectedValue(
      new CmdPaletteApiError("Command palette unavailable", 503, "daemon")
    );
    const { result } = harness();
    await waitFor(() => expect(result.current.hydrating).toBe(false));
    expect(result.current.catalog).toBeNull();
    expect(result.current.stale).toBe(false);
    expect(result.current.daemonReachable).toBe(false);
  });

  it("Should never hydrate one profile's catalog into another (IT-091)", async () => {
    // Marketing's offline record exists; default's does not. Entering default
    // must render nothing cached rather than Marketing's rows.
    await writeCatalogRecord(
      toCatalogRecord(
        WORKSPACE,
        "marketing",
        catalog("sha256:marketing", ["Marketing row"], "marketing")
      )
    );
    vi.spyOn(api, "listCmdPaletteCommands").mockRejectedValue(
      new CmdPaletteApiError("Command palette unavailable", 503, "daemon")
    );
    const { result } = harness();
    await waitFor(() => expect(result.current.hydrating).toBe(false));
    expect(result.current.catalog).toBeNull();

    // The record is still there for the profile that wrote it.
    await expect(readCatalogRecord(WORKSPACE, "marketing")).resolves.toMatchObject({
      catalogRevision: "sha256:marketing",
      profileKey: "marketing",
    });
    await dropCatalogRecord(WORKSPACE, "marketing");
  });

  it("Should send the profile lens with every catalog read (IT-091)", async () => {
    const list = vi
      .spyOn(api, "listCmdPaletteCommands")
      .mockResolvedValue(catalog("sha256:live", ["Live row"]));
    const { result } = harness();
    await waitFor(() => expect(result.current.daemonReachable).toBe(true));
    expect(list).toHaveBeenCalledWith(WORKSPACE, "default", CLIENT, expect.any(AbortSignal));
  });
});
