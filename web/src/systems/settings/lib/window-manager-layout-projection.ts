import type {
  WindowManagerLayoutDocument,
  WindowManagerLayoutState,
} from "./window-manager-layout-types";
import type { WindowManagerSnapshot } from "@/systems/os";

type WorkspaceOverrides = WindowManagerSnapshot["overrides"];

/**
 * The snapshot parser hands overrides over in camelCase; a layout document
 * carries them in wire form. Only kept keys are written back — retired ones
 * (`_spec.md` §Config Lifecycle) are ignored by the daemon and never re-sent.
 */
function overridesToWire(overrides: WorkspaceOverrides): Record<string, unknown> {
  const wire: Record<string, unknown> = {
    new_window_policy: overrides.newWindowPolicy,
    small_viewport_policy: overrides.smallViewportPolicy,
    focus_policy: overrides.focusPolicy,
    focus_wrap: overrides.focusWrap,
    focus_follows_pointer: overrides.focusFollowsPointer,
    swap_modifier: overrides.swapModifier,
    history_limit: overrides.historyLimit,
    nav_stack_limit: overrides.navStackLimit,
    closed_entry_limit: overrides.closedEntryLimit,
    desktop_transition: overrides.desktopTransition,
    gaps: overrides.gaps ? { ...overrides.gaps } : undefined,
    shortcuts: overrides.shortcuts ? structuredClone(overrides.shortcuts) : undefined,
    global_shortcuts: overrides.globalShortcuts ? { ...overrides.globalShortcuts } : undefined,
  };
  return Object.fromEntries(Object.entries(wire).filter(([, value]) => value !== undefined));
}

function projectDocument(snapshot: WindowManagerSnapshot): WindowManagerLayoutDocument {
  return {
    version: 6,
    workspaceId: snapshot.workspaceId,
    desktops: structuredClone(snapshot.desktops),
    windows: structuredClone(snapshot.windows),
    overrides: overridesToWire(snapshot.overrides),
  };
}

/** Mutable editor projection derived atomically from one daemon snapshot. */
export function windowManagerSnapshotToLayoutState(
  snapshot: WindowManagerSnapshot
): WindowManagerLayoutState {
  return { revision: snapshot.revision, document: projectDocument(snapshot) };
}
