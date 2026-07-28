import { createContext } from "react";

import type { RoutingCoordinator } from "../lib/routing-coordinator";
import type { WindowManagerController } from "../lib/os-types";

export interface OsShellHandle {
  store: WindowManagerController;
  manager: WindowManagerController;
  coordinator: RoutingCoordinator;
}

export const OsShellContext = createContext<OsShellHandle | null>(null);
