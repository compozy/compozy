import { buildPaletteRegistry } from "../lib/cmd-palette-registry";
import type { PaletteRegistry } from "../lib/cmd-palette-types";
import type { WindowManagerAttachedClientView } from "../lib/window-manager-types";
import { useCmdPaletteCatalog } from "./use-cmd-palette-catalog";
import { useCmdPaletteContext } from "./use-cmd-palette-context";
import { backgroundStreamsWithinConnectionBudget } from "../lib/background-stream-budget";
import { useCmdPaletteStream } from "./use-cmd-palette-stream";
import { useDesktop } from "./use-desktop";

export interface UseCmdPaletteRegistryOptions {
  readonly workspaceId: string | null;
  /** The attachment this projection speaks for; null until registration lands. */
  readonly client: WindowManagerAttachedClientView | null;
  readonly enabled?: boolean;
}

/**
 * The single registry projection (ADR-001/ADR-006).
 *
 * Three inputs meet here and nowhere else: the daemon's structural catalog
 * (hydrated stale-while-revalidate), this client's volatile context (evaluated
 * locally so availability never round-trips), and the effective keymap the
 * catalog carries. Palette root, destination mode, menubar, cheatsheet and the
 * settings shortcut table all render this one object, which is what makes a
 * drift between them impossible rather than merely discouraged.
 *
 * The non-topology context keys are read back from the attachment the daemon
 * echoed, so the client evaluates the same values the daemon would (SI-17).
 */
export function useCmdPaletteRegistry({
  workspaceId,
  client,
  enabled = true,
}: UseCmdPaletteRegistryOptions): PaletteRegistry {
  const clientId = client?.clientId ?? null;
  const { catalog, daemonReachable, stale } = useCmdPaletteCatalog({
    workspaceId,
    clientId,
    enabled,
  });
  const streamWithinConnectionBudget = useDesktop(backgroundStreamsWithinConnectionBudget);
  useCmdPaletteStream({ workspaceId, enabled: enabled && streamWithinConnectionBudget });
  const daemonContext = client?.paletteContext;
  const context = useCmdPaletteContext({
    // `shell.desktop` is the daemon's own derivation from the attachment kind;
    // mirroring it keeps the two evaluators from disagreeing about the surface.
    shellDesktop: daemonContext?.shellDesktop ?? false,
    scopeGlobal: daemonContext?.scopeGlobal ?? false,
    workspaceTrusted: daemonContext?.workspaceTrusted ?? true,
    focusedSessionState: daemonContext?.focusedSessionState ?? "",
    attached: client !== null,
  });
  return buildPaletteRegistry({
    catalog,
    context,
    daemonReachable,
    stale,
    platform: typeof navigator === "undefined" ? "" : navigator.platform,
  });
}
