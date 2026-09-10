import { frameForWindow } from "./group-projection";
import type { OsCloseScope, OsDesktopRuntimeStore, OsWindow } from "./os-types";

/** Mirrors the daemon's close scopes, including the pinned-tab boundary. */
export function windowCloseTargets(
  state: OsDesktopRuntimeStore,
  windowId: string,
  scope: OsCloseScope
): readonly OsWindow[] {
  const window = state.windows[windowId];
  if (!window) return [];
  if (scope === "tab") return window.pinned ? [] : [window];
  const frame = frameForWindow(state.frames, windowId);
  if (!frame?.stackId) return scope === "group" ? [window] : [];
  const selectedIndex = frame.members.indexOf(windowId);
  return frame.members.flatMap((id, index) => {
    const member = state.windows[id];
    if (!member) return [];
    if (scope === "group") return [member];
    return id === windowId || member.pinned || (scope === "right" && index <= selectedIndex)
      ? []
      : [member];
  });
}

export type WindowCloseGuard = (
  targets: readonly OsWindow[],
  isCurrent: () => boolean
) => Promise<boolean>;
