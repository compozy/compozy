"use client";

import { useEffect, useState } from "react";

/** Within this distance of its end, a page has no room left to bring a heading up. */
const END_SLACK_PX = 2;

/**
 * The id of the section being read, for an "On this page" rail: the last heading that crossed the
 * top third. A page scrolled to its end makes its last heading current, even when too little text
 * follows it to scroll it up that far.
 */
export function useActiveHeading(ids: readonly string[]) {
  const [active, setActive] = useState<string | null>(ids[0] ?? null);
  useEffect(() => {
    const update = () => {
      const end = document.documentElement.scrollHeight - END_SLACK_PX;
      if (window.scrollY > 0 && window.scrollY + window.innerHeight >= end) {
        setActive(ids.at(-1) ?? null);
        return;
      }
      let current = ids[0] ?? null;
      for (const id of ids) {
        const element = document.getElementById(id);
        if (element && element.getBoundingClientRect().top < window.innerHeight * 0.33)
          current = id;
      }
      setActive(current);
    };
    update();
    window.addEventListener("scroll", update, { passive: true });
    window.addEventListener("resize", update, { passive: true });
    return () => {
      window.removeEventListener("scroll", update);
      window.removeEventListener("resize", update);
    };
  }, [ids]);
  return active;
}
