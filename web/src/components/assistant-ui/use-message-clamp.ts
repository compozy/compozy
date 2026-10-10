import { useLayoutEffect, useRef, useState } from "react";

// A long message clamps with a bottom mask at 176px: text is never truncated,
// the mask lifts on "Show more". Slack beyond the cap avoids flapping on rounding.
const MESSAGE_CLAMP_MAX_PX = 44 * 4;
const MESSAGE_CLAMP_SLACK_PX = 8;

/** The mask a clamped body wears: `max-h-44` with a 28px bottom fade. */
export const MESSAGE_CLAMPED_CLASS =
  "max-h-44 overflow-hidden [mask-image:linear-gradient(to_bottom,#000_calc(100%-28px),transparent)]";

export interface MessageClamp {
  contentRef: React.RefObject<HTMLDivElement | null>;
  /** The body is taller than the cap: the toggle exists. */
  clampable: boolean;
  /** The mask is on now. */
  clamped: boolean;
  expanded: boolean;
  toggle: () => void;
}

/**
 * The clamp shared by the operator bubble and the session message cards (S1,
 * S2): measures the body and keeps measuring as it reflows.
 */
export function useMessageClamp(): MessageClamp {
  const contentRef = useRef<HTMLDivElement | null>(null);
  const [clampable, setClampable] = useState(false);
  const [expanded, setExpanded] = useState(false);

  useLayoutEffect(() => {
    const node = contentRef.current;
    if (!node) return;
    const measure = () => {
      setClampable(node.scrollHeight > MESSAGE_CLAMP_MAX_PX + MESSAGE_CLAMP_SLACK_PX);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  return {
    contentRef,
    clampable,
    clamped: clampable && !expanded,
    expanded,
    toggle: () => setExpanded(value => !value),
  };
}
