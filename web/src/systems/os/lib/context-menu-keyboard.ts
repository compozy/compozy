import type * as React from "react";

/** Opens a Base UI context menu from the platform menu keys at the focused control. */
export function openContextMenuFromKeyboard(event: React.KeyboardEvent<HTMLElement>): void {
  if (event.key !== "ContextMenu" && !(event.shiftKey && event.key === "F10")) return;
  event.preventDefault();
  const focused = event.currentTarget.ownerDocument.activeElement;
  const anchor =
    focused instanceof HTMLElement && event.currentTarget.contains(focused)
      ? focused
      : event.currentTarget;
  const bounds = anchor.getBoundingClientRect();
  event.currentTarget.dispatchEvent(
    new MouseEvent("contextmenu", {
      bubbles: true,
      cancelable: true,
      clientX: bounds.left + bounds.width / 2,
      clientY: bounds.top + bounds.height / 2,
    })
  );
}
