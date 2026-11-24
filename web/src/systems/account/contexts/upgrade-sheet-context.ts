import { createContext } from "react";

import type { OpenUpgradeSheetOptions } from "../hooks/use-upgrade-sheet";

export interface UpgradeSheetActions {
  open: boolean;
  openUpgradeSheet: (options?: OpenUpgradeSheetOptions) => void;
}

export const UpgradeSheetContext = createContext<UpgradeSheetActions | null>(null);
