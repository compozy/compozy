import { use } from "react";

import { CmdPaletteRegistryContext } from "../contexts/cmd-palette-registry-context-value";
import type { PaletteRegistry, ResolvedPaletteCommand } from "../lib/cmd-palette-types";

/** The live projection. Outside the shell it reads as an empty catalog, never as a crash. */
export function usePaletteRegistry(): PaletteRegistry {
  return use(CmdPaletteRegistryContext);
}

/** One command by id, or null when the catalog does not carry it. */
export function usePaletteCommand(commandId: string): ResolvedPaletteCommand | null {
  return use(CmdPaletteRegistryContext).byId.get(commandId) ?? null;
}
