import { createStreamEventSource, type StreamEventSource } from "@/lib/ticketed-event-source";

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

export function cmdPaletteStreamUrl(workspaceId: string): string {
  return `/api/cmd-palette/stream?workspace=${encodeURIComponent(workspaceId.trim())}`;
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
 * dropped before any cache write, so an invalidation can never cross workspaces.
 */
export function openCmdPaletteStream(
  workspaceId: string,
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

  const source = eventSourceFactory(cmdPaletteStreamUrl(workspace));
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
