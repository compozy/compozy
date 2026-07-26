import type { ReactNode } from "react";
import { cn } from "@agh/ui/lib/utils";

type Background = "canvas" | "surface" | "deep";
type PadY = "md" | "lg" | "xl";

const BG_CLASS: Record<Background, string> = {
  canvas: "bg-canvas",
  surface: "bg-canvas-soft",
  deep: "bg-rail",
};

const PAD_Y_CLASS: Record<PadY, string> = {
  md: "py-14 md:py-20",
  lg: "py-20 md:py-28",
  xl: "py-24 md:py-32",
};

interface SectionFrameProps {
  id?: string;
  children: ReactNode;
  background?: Background;
  padY?: PadY;
  className?: string;
  /** When true, the section renders a top divider. */
  divided?: boolean;
  /** Optional aria-label for the section landmark. */
  ariaLabel?: string;
}

export function SectionFrame({
  id,
  children,
  background = "canvas",
  padY = "lg",
  className,
  divided = false,
  ariaLabel,
}: SectionFrameProps) {
  return (
    <section
      id={id}
      aria-label={ariaLabel}
      className={cn(
        "px-4",
        BG_CLASS[background],
        PAD_Y_CLASS[padY],
        divided && "border-t border-line",
        className
      )}
    >
      <div className="mx-auto max-w-site-layout-width">{children}</div>
    </section>
  );
}
