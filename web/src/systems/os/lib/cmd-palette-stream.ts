import { createStreamEventSource, type StreamEventSource } from "@/lib/ticketed-event-source";

import { CMD_PALETTE_AGGREGATE_LENS_KEY, cmdPaletteProfileKey } from "./cmd-palette-query-keys";

/**
 * The daemon's catalog-invalidation stream.
 *
 * The event is consumed through a NAMED listener, never `onmessage` (L-017):
 * `cmd_palette.catalog.changed` shares the socket with any other family the
 * daemon adds, and a bare message handler would swallow them all and refetch on
 * frames that mean nothing to the palette.
 */
export const CMD_PALETTE_CATALOG_CHANGED_EVENT = "cmd_palette.catalog.changed";

export type CmdPaletteStreamStatus = "live" | "stale";

export interface CmdPaletteCatalogChangedPayload {
  readonly workspace: string;
  readonly catalogRevision: string;
}

export type CmdPaletteEventSourceFactory = (url: string) => StreamEventSource;

export interface CmdPaletteStreamHandlers {
  /** One event → exactly one reconciliation converging on the revision. */
  onCatalogChanged: (payload: CmdPaletteCatalogChangedPayload) => void;
  /** A reconnect may have missed frames, so `open` reconciles unconditionally. */
  onReconcile: () => void;
  onStatusChange: (status: CmdPaletteStreamStatus) => void;
}

/**
 * The invalidation socket is scoped on both axes.
 *
 * A catalog revision is a fact about one workspace *under one lens* — an
 * extension enabled in one profile and absent in another produces different
 * revisions — so subscribing without the lens would have the daemon answer for
 * `default` while the operator watches another profile.
 */
export function cmdPaletteStreamUrl(workspaceId: string, profileKey: string): string {
  const params = new URLSearchParams({ workspace: workspaceId.trim() });
  const lens = cmdPaletteProfileKey(profileKey);
  if (lens === CMD_PALETTE_AGGREGATE_LENS_KEY) params.set("all_profiles", "true");
  else params.set("profile", lens);
  return `/api/cmd-palette/stream?${params.toString()}`;
}

function parseCatalogChanged(event: Event): CmdPaletteCatalogChangedPayload | null {
  if (!(event instanceof MessageEvent) || typeof event.data !== "string") return null;
  try {
    const payload = JSON.parse(event.data) as Partial<{
      workspace: string;
      catalog_revision: string;
    }>;
    const workspace = payload.workspace?.trim() ?? "";
    const catalogRevision = payload.catalog_revision?.trim() ?? "";
    if (workspace === "" || catalogRevision === "") return null;
    return { workspace, catalogRevision };
  } catch {
    return null;
  }
}

/**
 * Opens the stream and returns its teardown. Frames for another workspace are
 * dropped before any cache write, so an invalidation can never cross workspaces;
 * the lens is carried on the subscription itself, so the daemon never sends this
 * client another profile's revisions in the first place.
 */
export function openCmdPaletteStream(
  workspaceId: string,
  profileKey: string,
  handlers: CmdPaletteStreamHandlers,
  eventSourceFactory: CmdPaletteEventSourceFactory = createStreamEventSource
): () => void {
  const workspace = workspaceId.trim();
  const handleOpen: EventListener = () => {
    handlers.onStatusChange("live");
    handlers.onReconcile();
  };
  const handleError: EventListener = () => handlers.onStatusChange("stale");
  const handleCatalogChanged: EventListener = event => {
    const payload = parseCatalogChanged(event);
    if (payload === null || payload.workspace !== workspace) return;
    handlers.onCatalogChanged(payload);
  };

  const source = eventSourceFactory(cmdPaletteStreamUrl(workspace, profileKey));
  const detach = () => {
    source.removeEventListener("open", handleOpen);
    source.removeEventListener("error", handleError);
    source.removeEventListener(CMD_PALETTE_CATALOG_CHANGED_EVENT, handleCatalogChanged);
  };
  try {
    source.addEventListener("open", handleOpen);
    source.addEventListener("error", handleError);
    source.addEventListener(CMD_PALETTE_CATALOG_CHANGED_EVENT, handleCatalogChanged);
  } catch (error) {
    detach();
    source.close();
    throw error;
  }

  return () => {
    detach();
    source.close();
  };
}
