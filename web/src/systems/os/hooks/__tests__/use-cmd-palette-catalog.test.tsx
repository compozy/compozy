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
const CLIENT = "client-1";

function catalog(revision: string, titles: readonly string[]): CmdPaletteCatalogResponse {
  return {
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

function harness() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
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
    await dropCatalogRecord(WORKSPACE);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should render the last-known catalog before the daemon answers, then reconcile to the fetched revision", async () => {
    await writeCatalogRecord(toCatalogRecord(WORKSPACE, catalog("sha256:cached", ["Cached row"])));
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
      expect((await readCatalogRecord(WORKSPACE))?.catalogRevision).toBe("sha256:persisted")
    );
    const stored = await readCatalogRecord(WORKSPACE);
    for (const command of stored?.commands ?? []) {
      expect(command).not.toHaveProperty("available");
      expect(command).not.toHaveProperty("reason");
    }
  });

  it("Should keep the cached catalog and report the daemon unreachable when the fetch fails", async () => {
    await writeCatalogRecord(toCatalogRecord(WORKSPACE, catalog("sha256:cached", ["Cached row"])));
    vi.spyOn(api, "listCmdPaletteCommands").mockRejectedValue(
      new CmdPaletteApiError("Command palette unavailable", 503, "daemon")
    );
    const { result } = harness();
    await waitFor(() => expect(result.current.catalog?.catalogRevision).toBe("sha256:cached"));
    expect(result.current.daemonReachable).toBe(false);
    expect(result.current.stale).toBe(true);
  });

  it("Should stop reporting the daemon reachable once a refetch fails mid-session", async () => {
    const list = vi
      .spyOn(api, "listCmdPaletteCommands")
      .mockResolvedValue(catalog("sha256:live", ["Live row"]));
    const { result, queryClient } = harness();
    await waitFor(() => expect(result.current.daemonReachable).toBe(true));

    // The daemon goes away; its last answer is still the best structure to
    // render, but nothing may claim it is reachable.
    list.mockRejectedValue(new CmdPaletteApiError("Command palette unavailable", 503, "daemon"));
    await queryClient.refetchQueries();

    await waitFor(() => expect(result.current.daemonReachable).toBe(false));
    expect(result.current.stale).toBe(true);
    expect(result.current.catalog?.commands.map(command => command.title)).toEqual(["Live row"]);
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
});
