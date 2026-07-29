import { useEffect, useRef, useState } from "react";

/** DOM ownership and focus restoration for the permission reject split menu. */
export function useRejectMenuElements() {
  const [menuOpen, setMenuOpen] = useState(false);
  const [splitElement, setSplitElement] = useState<HTMLDivElement | null>(null);
  const [triggerElement, setTriggerElement] = useState<HTMLButtonElement | null>(null);
  const [menuItemElement, setMenuItemElement] = useState<HTMLButtonElement | null>(null);
  const wasOpen = useRef(false);

  useEffect(() => {
    if (menuOpen) {
      menuItemElement?.focus();
    } else if (wasOpen.current) {
      triggerElement?.focus();
    }
    wasOpen.current = menuOpen;
  }, [menuItemElement, menuOpen, triggerElement]);

  return {
    menuItemRef: setMenuItemElement,
    menuOpen,
    setMenuOpen,
    splitElement,
    splitRef: setSplitElement,
    triggerRef: setTriggerElement,
  };
}
