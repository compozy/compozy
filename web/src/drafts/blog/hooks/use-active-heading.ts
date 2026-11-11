import { useEffect, useState } from "react";

/** The id of the section currently being read: the last heading that crossed the top third. */
export function useActiveHeading(ids: readonly string[]) {
  const [active, setActive] = useState(ids[0] ?? null);
  useEffect(() => {
    const update = () => {
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
    return () => window.removeEventListener("scroll", update);
  }, [ids]);
  return active;
}
