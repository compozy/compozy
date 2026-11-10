"use client";

import { usePathname } from "next/navigation";
import { useEffect, useRef, useState } from "react";

/** The small-screen site menu: Escape closes it and returns focus to its button; navigation closes it. */
export function useSiteMenu() {
  const pathname = usePathname();
  const [openOn, setOpenOn] = useState<string | null>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const open = openOn === pathname;
  useEffect(() => {
    if (!open) return undefined;
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      setOpenOn(null);
      buttonRef.current?.focus();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open]);
  return {
    pathname,
    open,
    buttonRef,
    close: () => setOpenOn(null),
    toggle: () => setOpenOn(open ? null : pathname),
  };
}
