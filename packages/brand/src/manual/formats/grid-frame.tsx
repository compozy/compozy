// A canvas drawn to ratio with a Visual, a headline in the quiet zone and the grid on top:
// margin (dashed), columns (tinted) and the story bands when asked.
import { cn } from "@compozy/ui/utils";

import { Visual } from "../art/visual";
import type { VisualName } from "../art/visuals";
import { columnsOf, gutterOf, marginOf, type Canvas } from "./data";

interface GridFrameProps {
  c: Canvas;
  visual?: VisualName;
  x?: number;
  y?: number;
  head?: string;
  kicker?: string;
  cols?: boolean;
  maxH?: number;
  bands?: [top: number, bottom: number];
}

/** red hatching for areas the platform covers */
export const HATCH =
  "bg-[repeating-linear-gradient(45deg,color-mix(in_oklab,var(--color-destructive)_35%,transparent)_0_6px,transparent_6px_12px)]";

const SCRIM_LAND =
  "linear-gradient(90deg, var(--color-rail) 0%, color-mix(in oklab, var(--color-rail) 80%, transparent) 42%, transparent 70%)";
const SCRIM_PORT =
  "linear-gradient(0deg, var(--color-rail) 0%, color-mix(in oklab, var(--color-rail) 85%, transparent) 38%, transparent 64%)";

export function GridFrame({
  c,
  visual = "swarm",
  x,
  y,
  head = "Agents that keep running after the tab closes.",
  kicker = "Release 0.6",
  cols = true,
  maxH = 420,
  bands,
}: GridFrameProps) {
  const m = marginOf(c);
  const n = columnsOf(c);
  const g = gutterOf(c);
  const mx = (m / c.w) * 100;
  const my = (m / c.h) * 100;
  const colW = ((c.w - 2 * m - (n - 1) * g) / c.w) * 100;
  const gw = (g / c.w) * 100;
  const portrait = c.h > c.w;
  const inset = `${my}% ${mx}%`;
  return (
    <div
      className="relative mx-auto w-full overflow-hidden rounded-[14px] bg-rail shadow-[0_0_0_1px_var(--b-line)] [container-type:inline-size]"
      style={{ aspectRatio: `${c.w}/${c.h}`, maxWidth: Math.round((maxH * c.w) / c.h) }}
    >
      <Visual name={visual} x={x ?? (portrait ? 0.7 : 0.86)} y={y ?? (portrait ? 0.3 : 0.5)} />
      <div
        className="absolute inset-0"
        style={{ background: portrait ? SCRIM_PORT : SCRIM_LAND }}
      />
      {cols &&
        Array.from({ length: n }, (_, i) => (
          <span
            key={i}
            className="absolute bg-brand-volt/16"
            style={{
              top: `${my}%`,
              bottom: `${my}%`,
              left: `${mx + i * (colW + gw)}%`,
              width: `${colW}%`,
            }}
          />
        ))}
      <span className="absolute border border-dashed border-brand-cyan" style={{ inset }} />
      <div className="absolute flex flex-col justify-between" style={{ inset }}>
        <img
          src="/brand/svg/compozy-lockup-dark.svg"
          alt=""
          className="w-auto self-start"
          style={{
            height: `${portrait ? 4.6 : 2.6}cqi`,
            marginTop: bands ? `${((bands[0] - my) * c.h) / c.w}cqi` : undefined,
          }}
        />
        <div
          style={
            bands ? { marginBottom: `${(bands[1] * c.h) / c.w - (my * c.h) / c.w}cqi` } : undefined
          }
        >
          <p
            className="m-0 mono-label text-brand-cyan"
            style={{ fontSize: `${portrait ? 3.4 : 1.6}cqi`, letterSpacing: ".12em" }}
          >
            {kicker}
          </p>
          <p
            className="m-0 font-display font-normal"
            style={{
              fontSize: `${portrait ? 8.4 : 5}cqi`,
              lineHeight: 1.05,
              letterSpacing: "-.03em",
              maxWidth: `${portrait ? 9 : 11}em`,
              marginTop: "1.4cqi",
            }}
          >
            {head}
          </p>
        </div>
      </div>
      {bands &&
        (
          [
            ["top", bands[0]],
            ["bottom", bands[1]],
          ] as const
        ).map(([pos, hh]) => (
          <span
            key={pos}
            className={cn("absolute inset-x-0", HATCH)}
            style={{ [pos]: 0, height: `${hh}%` }}
          />
        ))}
      <span className="absolute top-1.5 right-1.5 rounded bg-rail/85 px-1.5 font-mono text-[10px] text-muted-foreground">
        {n} col · {m} px · {g} px
      </span>
    </div>
  );
}
