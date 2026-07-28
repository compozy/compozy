import { useAtom } from "@xstate/store-react";

import { useOsShell } from "./use-os-shell";
import type { OsDesktopRuntimeStore } from "../lib/os-types";

/** Selector-scoped read of the Query-derived desktop projection. */
export function useDesktop<T>(
  selector: (state: OsDesktopRuntimeStore) => T,
  compare?: (left: T | undefined, right: T) => boolean
): T {
  const { projection } = useOsShell();
  return useAtom(projection, selector, compare);
}
