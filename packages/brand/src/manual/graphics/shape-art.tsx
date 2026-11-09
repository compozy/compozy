// The six shape tiles of Graphic system → Shapes, drawn from @compozy/ui colour tokens.
import type { ComponentType } from "react";

export type ShapeKind = "dot" | "contour" | "bar" | "hairline" | "pill" | "panel";

const COLS = 14;
const dots = Array.from({ length: 9 * COLS }, (_, k) => ({ r: Math.floor(k / COLS), c: k % COLS }));
const lit = new Set([3 * 14 + 6, 3 * 14 + 7, 4 * 14 + 7, 5 * 14 + 8, 5 * 14 + 9, 6 * 14 + 9]);
const svg = "absolute inset-0 h-full w-full";

function DotTile() {
  return (
    <svg viewBox="0 0 400 300" className={svg} aria-hidden="true">
      {dots.map(({ r, c }) => {
        const k = r * COLS + c;
        const y = 60 + r * 24;
        const sc = 0.55 + r * 0.06;
        const x = 200 + (c - 6.5) * 26 * sc * 1.6;
        return lit.has(k) ? (
          <circle key={k} cx={x} cy={y} r={3.6 * sc + 0.6} className="fill-brand-cyan" />
        ) : (
          <circle key={k} cx={x} cy={y} r={2.2 * sc} className="fill-disabled" />
        );
      })}
    </svg>
  );
}

function ContourTile() {
  return (
    <svg viewBox="0 0 400 300" className={svg} fill="none" aria-hidden="true">
      {[0, 1, 2, 3, 4, 5, 6].map(k => (
        <path
          key={k}
          d={`M0 ${70 + k * 28} C 90 ${50 + k * 28}, 150 ${110 + k * 24}, 230 ${80 + k * 28} S 360 ${40 + k * 28}, 400 ${70 + k * 28}`}
          className={k === 3 ? "stroke-brand-cyan" : "stroke-line-strong"}
          strokeWidth="1"
        />
      ))}
    </svg>
  );
}

function BarTile() {
  return (
    <svg viewBox="0 0 400 300" className={svg} aria-hidden="true">
      <defs>
        <linearGradient id="barg" x1="0" x2="1" y1="1" y2="0">
          <stop offset="0" style={{ stopColor: "var(--color-brand-volt)" }} />
          <stop offset="1" style={{ stopColor: "var(--color-brand-cyan)" }} />
        </linearGradient>
      </defs>
      {Array.from({ length: 36 }, (_, k) => {
        const h = 30 + ((k * 37) % 11) * 11 + (k > 26 ? 40 : 0);
        return (
          <rect
            key={k}
            x={34 + k * 9.6}
            y={250 - h}
            width="3"
            height={h}
            rx="1.5"
            fill={k > 26 ? "url(#barg)" : undefined}
            className={k > 26 ? undefined : "fill-line-strong"}
          />
        );
      })}
    </svg>
  );
}

function HairlineTile() {
  return (
    <svg viewBox="0 0 400 300" className={svg} aria-hidden="true">
      {[1, 2, 3, 4, 5, 6, 7].map(k => (
        <line
          key={`v${k}`}
          x1={k * 50}
          x2={k * 50}
          y1="0"
          y2="300"
          className="stroke-line"
          strokeWidth="1"
        />
      ))}
      {[1, 2, 3, 4, 5].map(k => (
        <line
          key={`h${k}`}
          y1={k * 50}
          y2={k * 50}
          x1="0"
          x2="400"
          className="stroke-line"
          strokeWidth="1"
        />
      ))}
      <line x1="50" x2="350" y1="150" y2="150" className="stroke-brand-cyan" strokeWidth="1" />
      <circle cx="50" cy="150" r="3" className="fill-brand-cyan" />
      <circle cx="350" cy="150" r="3" fill="none" className="stroke-brand-cyan" />
    </svg>
  );
}

function PillTile() {
  return (
    <div className="absolute inset-0 grid place-content-center justify-items-center gap-3">
      <span className="rounded-full border border-line-strong px-3.5 py-1.5 font-mono text-[13px] text-brand-cyan">
        ● 12 working
      </span>
      <span className="rounded-full border border-line-strong px-3.5 py-1.5 font-mono text-[13px] text-muted-foreground">
        always-on
      </span>
      <span className="rounded-full px-3.5 py-1.5 font-mono text-[13px] text-on-gradient [background:var(--gradient-brand)]">
        run_7f2
      </span>
    </div>
  );
}

function PanelTile() {
  return (
    <div className="absolute inset-[12%] grid grid-cols-[1fr_1.5fr] gap-2.5">
      <span className="rounded-[10px] border border-line-strong bg-canvas" />
      <span className="grid grid-rows-2 gap-2.5">
        <span className="rounded-[14px] border border-line-strong bg-canvas" />
        <span className="rounded-[14px] border border-line bg-elevated" />
      </span>
    </div>
  );
}

const TILES: Record<ShapeKind, ComponentType> = {
  dot: DotTile,
  contour: ContourTile,
  bar: BarTile,
  hairline: HairlineTile,
  pill: PillTile,
  panel: PanelTile,
};

export function ShapeTile({ kind }: { kind: ShapeKind }) {
  const Tile = TILES[kind];
  return <Tile />;
}

/** Construction drawing of the dot field: a perspective grid with a few lit points. */
export function DotFieldConstruction() {
  const rows = [0, 1, 2, 3, 4, 5, 6, 7];
  return (
    <svg
      viewBox="0 0 400 300"
      className="mt-5 w-full rounded-[14px] bg-rail"
      role="img"
      aria-label="Dot field construction: a regular grid seen in perspective, pitch shrinking toward the horizon, a few dots lit"
    >
      {rows.map(r => {
        const y = 90 + r * 24;
        const sc = 0.5 + r * 0.07;
        return (
          <line
            key={r}
            x1={200 - 190 * sc}
            x2={200 + 190 * sc}
            y1={y}
            y2={y}
            className="stroke-line-strong"
            strokeDasharray="3 4"
          />
        );
      })}
      <line
        x1="200"
        x2="200"
        y1="60"
        y2="270"
        className="stroke-line-strong"
        strokeDasharray="3 4"
      />
      {rows.flatMap(r =>
        [-3, -2, -1, 0, 1, 2, 3].map(c => {
          const y = 90 + r * 24;
          const sc = 0.5 + r * 0.07;
          const on = (r === 5 && c === 1) || (r === 6 && c === 2) || (r === 4 && c === 0);
          return (
            <circle
              key={`${r}:${c}`}
              cx={200 + c * 46 * sc}
              cy={y}
              r={on ? 4 : 2.2 * sc + 0.6}
              className={on ? "fill-brand-cyan" : "fill-disabled"}
            />
          );
        })
      )}
      <text x="16" y="28" className="fill-muted-foreground font-mono" fontSize="11">
        pitch 46 px at the front · ×0.5 at the horizon
      </text>
      <text x="16" y="286" className="fill-muted-foreground font-mono" fontSize="11">
        dot 2 px · lit 4 px · 1 in 10 lit
      </text>
    </svg>
  );
}
