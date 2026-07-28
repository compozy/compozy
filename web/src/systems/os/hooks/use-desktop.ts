import { useStore } from "zustand";

import { useOsShell } from "./use-os-shell";
import type { OsDesktopRuntimeStore } from "../lib/os-types";

/** Selector-scoped read of the Query-derived desktop projection. */
export function useDesktop<T>(selector: (state: OsDesktopRuntimeStore) => T): T {
  const { store } = useOsShell();
  return useStore(store, selector);
}
