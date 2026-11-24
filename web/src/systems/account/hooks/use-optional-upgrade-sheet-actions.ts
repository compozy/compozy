import { useContext } from "react";

import { UpgradeSheetContext, type UpgradeSheetActions } from "../contexts/upgrade-sheet-context";

/** Returns the shell-level Upgrade sheet trigger when mounted inside `AccountUpgradeSheetProvider`. */
export function useOptionalUpgradeSheetActions(): UpgradeSheetActions | null {
  return useContext(UpgradeSheetContext);
}
