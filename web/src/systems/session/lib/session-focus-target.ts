/**
 * Stable focus hooks for host logic, apart from test ids: a host moves focus to a session's
 * composer or its pending permission without reaching into either component.
 */
export const SESSION_FOCUS_TARGET = {
  composer: "session-composer",
  permissionAllowOnce: "permission-allow-once",
} as const;

export type SessionFocusTarget = keyof typeof SESSION_FOCUS_TARGET;

/** The first `target` under `root`; for the composer, its editable surface when it has one. */
export function findSessionFocusTarget(
  root: ParentNode | null | undefined,
  target: SessionFocusTarget
): HTMLElement | null {
  const element =
    root?.querySelector<HTMLElement>(`[data-focus-target="${SESSION_FOCUS_TARGET[target]}"]`) ??
    null;
  return element?.querySelector<HTMLElement>('[contenteditable="true"]') ?? element;
}
