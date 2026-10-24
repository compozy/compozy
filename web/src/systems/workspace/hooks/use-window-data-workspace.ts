import { createContext, useContext } from "react";

import type { DataScope } from "../lib/destination";
import { windowDataScope } from "../lib/window-data-scope";
import { useDataScope } from "./use-active-workspace";

/** The mounted window's partition and serialized binding, provided by the OS window frame. */
export interface WindowDataBinding {
  partition: string | null;
  binding: unknown;
}

/**
 * Provided per retained OS window (alongside its worktree scope), so domain
 * systems read their window's binding without importing `os`.
 */
export const WindowDataBindingContext = createContext<WindowDataBinding | null>(null);

/**
 * The data scope of the window rendering this subtree: its serialized binding
 * when present (`"<id>"` → project, `"@user"` → User), otherwise its partition
 * (`global` → all, project → that project, `user` → User). Outside a window
 * it is the menubar destination. `null` while unsettled.
 */
export function useWindowDataWorkspace(): DataScope | null {
  const frame = useContext(WindowDataBindingContext);
  const destination = useDataScope({ enabled: frame === null });
  return frame === null ? destination : windowDataScope(frame.partition, frame.binding);
}

/**
 * Search that keeps an in-window item navigation on the mounted window's
 * serialized binding (`{ workspace }`), so following a link inside a bound
 * window never drops back to the partition's data. Unbound windows get `{}`.
 */
export function useWindowBindingSearch(): { workspace?: string } {
  const binding = useContext(WindowDataBindingContext)?.binding;
  return typeof binding === "string" && binding.trim() !== "" ? { workspace: binding.trim() } : {};
}
