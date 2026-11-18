// Renders one visual of the Compozy system filling its positioned parent.
//   <Visual name="swarm" />                   atmosphere, picks the portrait crop when the box is portrait
//   <Visual name="swarm" x={0.8} y={0.4} />   focal point (object-position), 0..1
//   <Visual name="desktop" inline ratio="16/10" />  block flow with an aspect ratio
//   <Visual name="desktop" />                 product capture in perspective (cover, background)
//   <Visual name="desktop" inline />          product capture as a flat crop
//   <Visual name="lanes" theme="light" />     token diagram, on paper
import { cn } from "@compozy/ui/utils";

import { BarsDiagram } from "./diagrams/bars-diagram";
import { LanesDiagram } from "./diagrams/lanes-diagram";
import { StackDiagram } from "./diagrams/stack-diagram";
import { ProductShot } from "./product-shot";
import {
  atmosphereSrcSet,
  visualByName,
  VISUALS_BASE,
  type DiagramName,
  type ProductShotName,
  type VisualInfo,
  type VisualName,
} from "./visuals";

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
  const focusX = x ?? v.focus[0];
  const focusY = y ?? v.focus[1];
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
      <VisualBody
        info={v}
        name={name}
        x={focusX}
        y={focusY}
        theme={theme}
        inline={inline}
        sizes={sizes}
      />
    </div>
  );
}

interface VisualBodyProps {
  info: VisualInfo;
  name: VisualName;
  x: number;
  y: number;
  theme: "dark" | "light";
  inline: boolean;
  sizes: string;
}

/** One render branch per visual family. */
function VisualBody({ info, name, x, y, theme, inline, sizes }: VisualBodyProps) {
  if (info.family === "atmosphere") {
    return (
      <Atmosphere
        name={name}
        portrait={info.portrait}
        objectPosition={`${pct(x)} ${pct(y)}`}
        sizes={sizes}
      />
    );
  }
  if (info.family === "product") {
    return (
      <ProductShot
        name={name as ProductShotName}
        theme={theme}
        x={x}
        y={y}
        mode={inline ? "flat" : "perspective"}
        alt={inline ? `${info.title}: ${info.idea}` : ""}
      />
    );
  }
  return <Diagram name={name as DiagramName} x={x} y={y} />;
}

/** Landscape crop, plus the portrait crop when the visual ships one (CSS picks by box shape). */
function Atmosphere({
  name,
  portrait,
  objectPosition,
  sizes,
}: {
  name: VisualName;
  portrait?: boolean;
  objectPosition: string;
  sizes: string;
}) {
  return (
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
      {portrait && (
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
  );
}

function Diagram({ name, x, y }: { name: DiagramName; x: number; y: number }) {
  if (name === "stack") return <StackDiagram x={x} y={y} />;
  if (name === "lanes") return <LanesDiagram />;
  return <BarsDiagram />;
}
