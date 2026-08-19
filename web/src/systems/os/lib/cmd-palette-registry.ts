import { resolvePaletteAvailability } from "./cmd-palette-availability";
import type { CmdPaletteContextSnapshot } from "./cmd-palette-context";
import type {
  CmdPaletteStructuralCatalog,
  PaletteRegistry,
  ResolvedPaletteCommand,
} from "./cmd-palette-types";
import { primaryShortcutModifier, shortcutLabel } from "./window-manager-shortcuts";

export const EMPTY_PALETTE_REGISTRY: PaletteRegistry = {
  commands: [],
  byId: new Map(),
  sources: [],
  catalogRevision: "",
  stale: false,
  daemonReachable: false,
};

export interface PaletteRegistryInput {
  readonly catalog: CmdPaletteStructuralCatalog | null;
  readonly context: CmdPaletteContextSnapshot | null;
  readonly daemonReachable: boolean;
  readonly stale: boolean;
  /** Platform string used to render ⌘ vs ⌃ in chord badges. */
  readonly platform: string;
}

/**
 * Builds the one client registry every surface renders from (`_uiux.md`
 * Component plan): the daemon's structure, judged against this client's
 * context, with the effective chord already formatted.
 *
 * Bindings arrive from the daemon keymap, so no surface can hold a chord
 * literal (BR-6), and because palette rows, menubar items, cheatsheet lines and
 * the settings table all read this object, they cannot disagree about a
 * command's id, label or chord (US-001.AC-4).
 */
export function buildPaletteRegistry({
  catalog,
  context,
  daemonReachable,
  stale,
  platform,
}: PaletteRegistryInput): PaletteRegistry {
  if (catalog === null) {
    return { ...EMPTY_PALETTE_REGISTRY, stale, daemonReachable };
  }
  const primaryModifier = primaryShortcutModifier(platform);
  const commands: ResolvedPaletteCommand[] = [];
  for (const command of catalog.commands) {
    const availability = resolvePaletteAvailability(command, context, { daemonReachable });
    if (!availability.visible) continue;
    commands.push({
      ...command,
      ...availability,
      chords: command.bindings.flatMap(binding => {
        const label = shortcutLabel(binding, primaryModifier);
        return label === "" ? [] : [label];
      }),
    });
  }
  return {
    commands,
    byId: new Map(commands.map(command => [command.id, command])),
    sources: catalog.sources,
    catalogRevision: catalog.catalogRevision,
    stale,
    daemonReachable,
  };
}
