/** A dark card with a soft light that follows the pointer. The position lives in CSS, so moving it never re-renders. */
import { cn } from "@compozy/ui";
import type { PointerEvent, ReactNode } from "react";

const HAIRLINE_MASK =
  "[-webkit-mask:linear-gradient(#000_0_0)_content-box,linear-gradient(#000_0_0)] [-webkit-mask-composite:xor] [mask:linear-gradient(#000_0_0)_content-box_exclude,linear-gradient(#000_0_0)]";

function trackPointer(event: PointerEvent<HTMLDivElement>) {
  const card = event.currentTarget;
  const box = card.getBoundingClientRect();
  card.style.setProperty("--spot-x", `${event.clientX - box.left}px`);
  card.style.setProperty("--spot-y", `${event.clientY - box.top}px`);
}

export function SpotlightCard({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      onPointerMove={trackPointer}
      className={cn(
        "group/spot relative isolate overflow-hidden bg-[#111111] shadow-[0_24px_48px_-24px_var(--dark-600)]",
        className
      )}
    >
      <span
        aria-hidden
        className="pointer-events-none absolute inset-0 -z-10 rounded-[inherit] opacity-0 transition-opacity duration-300 [background:radial-gradient(240px_circle_at_var(--spot-x)_var(--spot-y),var(--spot-teal),transparent_72%)] group-hover/spot:opacity-100"
      />
      {children}
      <span
        aria-hidden
        className={cn(
          "pointer-events-none absolute inset-0 z-[1] rounded-[inherit] p-px [background:var(--light-080)]",
          HAIRLINE_MASK
        )}
      />
    </div>
  );
}
