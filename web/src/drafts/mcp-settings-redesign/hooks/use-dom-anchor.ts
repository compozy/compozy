import { useEffect, useState } from "react";

/** The first element matching `selector`, polled per frame until the backdrop renders it. */
export function useDomAnchor(selector: string | undefined): Element | null {
  const [element, setElement] = useState<Element | null>(null);
  useEffect(() => {
    if (!selector) return undefined;
    let frame = 0;
    const find = () => {
      const found = document.querySelector(selector);
      if (found) setElement(found);
      else frame = requestAnimationFrame(find);
    };
    find();
    return () => cancelAnimationFrame(frame);
  }, [selector]);
  return element;
}
