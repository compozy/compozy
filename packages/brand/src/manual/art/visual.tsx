// Renders one visual of the Compozy system filling its positioned parent.
//   <Visual name="swarm" />                   atmosphere, picks the portrait crop when the box is portrait
//   <Visual name="swarm" x={0.8} y={0.4} />   focal point (object-position), 0..1
//   <Visual name="desktop" inline ratio="16/10" />  block flow with an aspect ratio
// Product shots and diagrams arrive with slice S3 (ADR-005); until then they render a marked slot.
import { cn } from "@compozy/ui/utils";

import { atmosphereSrcSet, visualByName, VISUALS_BASE, type VisualName } from "./visuals";

interface VisualProps {
  name: VisualName;
  x?: number;
  y?: number;
  theme?: "dark" | "light";
  className?: string;
  inline?: boolean;
  ratio?: string;
  sizes?: string;
}

const pct = (n: number) => `${Math.round(n * 100)}%`;

export function Visual({
  name,
  x,
  y,
  theme = "dark",
  className,
  inline = false,
  ratio = "16/9",
  sizes = "100vw",
}: VisualProps) {
  const v = visualByName(name);
  const objectPosition = `${pct(x ?? v.focus[0])} ${pct(y ?? v.focus[1])}`;
  return (
    <div
      data-visual={name}
      data-visual-family={v.family}
      className={cn(
        "cz-visual overflow-hidden",
        inline ? "relative block w-full" : "absolute inset-0",
        theme === "light" && "band-paper",
        className
      )}
      style={inline ? { aspectRatio: ratio } : undefined}
    >
      {v.family === "atmosphere" ? (
        <>
          <img
            className="cz-land"
            src={`${VISUALS_BASE}/${name}.jpg`}
            srcSet={atmosphereSrcSet(name, "land")}
            sizes={sizes}
            alt=""
            decoding="async"
            style={{ objectPosition }}
          />
          {v.portrait && (
            <img
              className="cz-port"
              src={`${VISUALS_BASE}/${name}-portrait.jpg`}
              srcSet={atmosphereSrcSet(`${name}-portrait`, "port")}
              sizes={sizes}
              alt=""
              decoding="async"
              style={{ objectPosition }}
            />
          )}
        </>
      ) : (
        <PendingVisualSlot title={v.title} family={v.family} name={name} />
      )}
    </div>
  );
}

/** Marked slot for a product shot or diagram that slice S3 fills (ADR-005). */
function PendingVisualSlot({
  title,
  family,
  name,
}: {
  title: string;
  family: string;
  name: string;
}) {
  return (
    <div
      data-visual-pending="S3"
      className="absolute inset-0 grid place-items-end justify-end bg-[radial-gradient(circle_at_75%_45%,color-mix(in_oklab,var(--color-brand-volt)_18%,transparent),transparent_62%)] p-[4%]"
    >
      <p className="m-0 rounded-md border border-dashed border-(--b-line) bg-(--b-surface) px-3 py-1.5 mono-label text-[11px] text-(--b-muted)">
        {family === "product" ? "Product shot" : "Diagram"} · {title} · {name}
      </p>
    </div>
  );
}
