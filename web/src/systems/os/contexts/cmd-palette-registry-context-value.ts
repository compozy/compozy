import { createContext } from "react";

import { EMPTY_PALETTE_REGISTRY } from "../lib/cmd-palette-registry";
import type { PaletteRegistry } from "../lib/cmd-palette-types";

export const CmdPaletteRegistryContext = createContext<PaletteRegistry>(EMPTY_PALETTE_REGISTRY);
