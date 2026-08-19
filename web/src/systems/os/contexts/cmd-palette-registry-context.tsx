import type { ReactNode } from "react";

import { CmdPaletteRegistryContext } from "./cmd-palette-registry-context-value";
import type { PaletteRegistry } from "../lib/cmd-palette-types";

/**
 * Injects the one registry projection into every surface that renders commands.
 *
 * Menubar items, the cheatsheet and the settings shortcut table sit at very
 * different depths of the shell; handing them the same object through context
 * is what lets them show identical ids, labels and chords for a command without
 * any of them owning a list (US-001.AC-4). Dispatch is deliberately *not* here:
 * it belongs to the shell body that owns the coordinators, and travels as props.
 */
export function CmdPaletteRegistryProvider({
  registry,
  children,
}: {
  registry: PaletteRegistry;
  children: ReactNode;
}) {
  return (
    <CmdPaletteRegistryContext.Provider value={registry}>
      {children}
    </CmdPaletteRegistryContext.Provider>
  );
}
