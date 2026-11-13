import { useState, type KeyboardEvent } from "react";

import type { DesktopBarControl } from "../lib/desktop-indicator-controls";

/**
 * One tab stop across the desktop bar's controls (roving tabindex): arrows, Home and End move
 * focus, and the stop follows the last focused control while it stays visible, else the active
 * desktop.
 */
export function useDesktopIndicatorRoving(
  controls: readonly DesktopBarControl[],
  activeDesktopId: string
) {
  const controlRefs = new Map<string, HTMLButtonElement>();
  const [rovingKey, setRovingKey] = useState<string>();
  const activeKey = `desktop:${activeDesktopId}`;
  const visibleKeys = new Set(controls.map(control => control.key));
  const tabStopKey = rovingKey && visibleKeys.has(rovingKey) ? rovingKey : activeKey;

  const focusControl = (position: number) => {
    const control = controls[position];
    if (!control) return;
    setRovingKey(control.key);
    controlRefs.get(control.key)?.focus();
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>, key: string) => {
    const position = controls.findIndex(control => control.key === key);
    if (position < 0) return;
    let nextPosition: number | null = null;
    if (event.key === "ArrowLeft") nextPosition = Math.max(0, position - 1);
    if (event.key === "ArrowRight") nextPosition = Math.min(controls.length - 1, position + 1);
    if (event.key === "Home") nextPosition = 0;
    if (event.key === "End") nextPosition = controls.length - 1;
    if (nextPosition === null) return;
    event.preventDefault();
    focusControl(nextPosition);
  };

  const controlProps = (control: DesktopBarControl) => ({
    ref: (node: HTMLButtonElement | null) => {
      if (node) controlRefs.set(control.key, node);
      else controlRefs.delete(control.key);
    },
    tabIndex: control.key === tabStopKey ? 0 : -1,
    onFocus: () => setRovingKey(control.key),
    onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) => handleKeyDown(event, control.key),
  });

  return { controlProps, rove: setRovingKey };
}
