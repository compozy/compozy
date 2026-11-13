import { createContext } from "react";

import type { OsTabBarModel } from "../hooks/use-os-tab-bar";

/**
 * The live tab bar, for what reads the whole bar only while it shows: a tab's open menu. Tabs
 * themselves take their own facts as props, so a switch re-renders the tabs it changed.
 */
export const OsTabBarContext = createContext<OsTabBarModel | null>(null);
