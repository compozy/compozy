import { useEffect, useRef, useState } from "react";

/**
 * Opens a native dialog as a modal for as long as it is mounted: the browser keeps focus inside
 * and makes the rest of the page inert. Focus returns to the control that opened it on unmount.
 */
export function useModalDialog() {
  const ref = useRef<HTMLDialogElement>(null);
  // Read during the first render, before anything inside the dialog can take focus.
  const [opener] = useState(() =>
    typeof document !== "undefined" && document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null
  );
  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (!dialog.open) {
      if (typeof dialog.showModal === "function") dialog.showModal();
      else dialog.setAttribute("open", "");
      // showModal focuses the first focusable element; a text field inside should win.
      dialog.querySelector<HTMLElement>("input")?.focus();
    }
    // No close() on cleanup: removing the element ends the modal, and close() would fire a
    // `close` event that the owner reads as "the user dismissed it".
    return () => {
      window.setTimeout(() => {
        if (!dialog.isConnected && opener?.isConnected) opener.focus();
      }, 0);
    };
  }, [opener]);
  return ref;
}
