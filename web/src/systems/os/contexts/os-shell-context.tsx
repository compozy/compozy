import { createContext } from "react";

import type { RoutingCoordinator } from "../lib/routing-coordinator";
import type { WindowManagerController } from "../lib/os-types";
import type { WindowManagerProjectionAtom } from "../lib/window-manager-projection";

export interface OsShellHandle {
  projection: WindowManagerProjectionAtom;
  manager: WindowManagerController;
  coordinator: RoutingCoordinator;
}

export const OsShellContext = createContext<OsShellHandle | null>(null);
