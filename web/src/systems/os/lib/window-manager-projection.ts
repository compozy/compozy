import { createAtom } from "@xstate/store";

import type { OsDesktopRuntimeStore } from "./os-types";

export function createWindowManagerProjectionAtom(initial: OsDesktopRuntimeStore) {
  return createAtom(initial);
}

export type WindowManagerProjectionAtom = ReturnType<typeof createWindowManagerProjectionAtom>;
