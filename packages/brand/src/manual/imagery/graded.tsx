// A photo with the Compozy grade applied live in CSS (see cssRecipe in data.ts).
// grade=false shows the untreated file, for before/after comparisons.
import { cn } from "@compozy/ui/utils";

const GRAIN =
  "url(\"data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='160' height='160'><filter id='n'><feTurbulence type='fractalNoise' baseFrequency='.9' numOctaves='2' stitchTiles='stitch'/><feColorMatrix values='0 0 0 0 .5 0 0 0 0 .5 0 0 0 0 .5 0 0 0 .5 0'/></filter><rect width='100%' height='100%' filter='url(%23n)'/></svg>\")";

interface GradedProps {
  src: string;
  alt: string;
  ratio?: string;
  pos?: string;
  grade?: boolean;
  className?: string;
}

export function Graded({
  src,
  alt,
  ratio = "4/5",
  pos = "50% 50%",
  grade = true,
  className,
}: GradedProps) {
  return (
    <div
      className={cn(
        "relative isolate overflow-hidden rounded-[14px] bg-(--b-card) shadow-[0_0_0_1px_var(--b-line)]",
        className
      )}
      style={{ aspectRatio: ratio }}
    >
      <img
        src={src}
        alt={alt}
        loading="lazy"
        decoding="async"
        className="block h-full w-full object-cover"
        style={{
          objectPosition: pos,
          filter: grade ? "saturate(.8) contrast(.92) brightness(1.03)" : undefined,
        }}
      />
      {grade && (
        <>
          <span className="pointer-events-none absolute inset-0 bg-brand-sky opacity-[.14] mix-blend-soft-light" />
          <span
            className="pointer-events-none absolute inset-0 mix-blend-lighten"
            style={{
              background: `var(--color-rail) ${GRAIN}`,
              backgroundBlendMode: "overlay",
            }}
          />
        </>
      )}
    </div>
  );
}
