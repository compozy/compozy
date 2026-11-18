// 01 Shapes: the vocabulary of the visual system, how to crop a visual, and what never to do.
import { cn } from "@compozy/ui/utils";

import { Visual } from "../art/visual";
import { cap, card, h3, lab, rules } from "../cls";
import { Section } from "../parts/section";
import { cropRules, shapeRules } from "./data";
import { DotFieldConstruction, ShapeTile, type ShapeKind } from "./shape-art";
import { SpecCard } from "../parts/spec-card";

const shapes: { name: string; use: string; kind: ShapeKind }[] = [
  {
    name: "Point",
    use: "The unit of Atmosphere: one agent, one run, one file. Dots sit on a regular grid; a few are lit.",
    kind: "dot",
  },
  {
    name: "Contour",
    use: "A 1 px line that follows a surface: terrain ridges, lane rules, the path a run travels. One may be lit.",
    kind: "contour",
  },
  {
    name: "Bar",
    use: "A run, a day, a count. 2 px wide at 1×, rounded ends; the live one carries the gradient.",
    kind: "bar",
  },
  {
    name: "Hairline",
    use: "1 px rules and grids in line colour. Structure for diagrams, tables and covers.",
    kind: "hairline",
  },
  {
    name: "Pill",
    use: "State and identifiers: status, counts, the OS chip. Radius 999 px, mono label.",
    kind: "pill",
  },
  {
    name: "Rounded panel",
    use: "Windows, cards, terminal panes, diagram nodes. Radius 10, 14 or 20 px by size.",
    kind: "panel",
  },
];

const proportions: [string, string][] = [
  ["Dot", "2 px grey, 4 px when lit; 1 in 10 lit at most"],
  ["Pitch", "8, 16 or 32 px on the 8 px scale; 0.5× at the horizon"],
  ["Bar", "2 px wide, 8 px gap, rounded ends, the live window in gradient"],
  ["Contour", "1 px, #262626 to #383838; one lit line per surface"],
  ["Gradient", "Volt #2F5BFF at the base to cyan #3DF2E0 at the lit end, at 45°, never reversed"],
  ["Line weight", "1 px on screen; 0.25 pt in print"],
  ["Panel radius", "10 px (controls) · 14 px (cards) · 20 px (sections)"],
];

const crops: { cap: string; name: "swarm" | "converge" | "terrain"; x: number; y: number }[] = [
  {
    cap: "Right edge: the mass leaves the frame on the right, the quiet area takes the left.",
    name: "swarm",
    x: 0.95,
    y: 0.5,
  },
  {
    cap: "Bottom edge: the stream rises from below, quiet area on top.",
    name: "converge",
    x: 0.5,
    y: 1,
  },
  {
    cap: "Left edge: the ridge runs under long text, kept dim by a scrim.",
    name: "terrain",
    x: 0.05,
    y: 0.6,
  },
];

type DontKind = "hue" | "two" | "stretch" | "text";
const donts: { cap: string; kind: DontKind }[] = [
  {
    cap: "Another hue. The art only ever carries the blue and cyan of the signal gradient.",
    kind: "hue",
  },
  { cap: "Two visuals in one piece. One piece, one visual.", kind: "two" },
  { cap: "Stretched to fit. The perspective breaks; change x and y instead.", kind: "stretch" },
  { cap: "Text on the lit part, no scrim. Contrast drops under 3:1.", kind: "text" },
];

function DontArt({ kind }: { kind: DontKind }) {
  if (kind === "hue")
    return (
      <div className="absolute inset-0" style={{ filter: "hue-rotate(150deg) saturate(1.4)" }}>
        <Visual name="swarm" x={0.7} y={0.5} />
      </div>
    );
  if (kind === "two")
    return (
      <>
        <Visual name="swarm" x={0.8} y={0.4} />
        <div className="absolute inset-0 mix-blend-screen [mask-image:linear-gradient(0deg,black_0%,transparent_70%)]">
          <Visual name="terrain" x={0.3} y={0.8} />
        </div>
      </>
    );
  if (kind === "stretch")
    return (
      <div className="absolute inset-0 origin-left" style={{ transform: "scaleX(1.7) scaleY(.6)" }}>
        <Visual name="converge" x={0.5} y={0.5} />
      </div>
    );
  return (
    <>
      <Visual name="cone" x={0.35} y={0.3} />
      <p className="absolute top-[18%] left-[12%] m-0 max-w-[8em] font-display text-[22px] leading-[1.05] font-normal tracking-[-0.03em] text-fg-strong">
        One system for every agent
      </p>
    </>
  );
}

function Vocabulary() {
  return (
    <>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {shapes.map((s, i) => (
          <div key={s.kind} className={card}>
            <div className="relative aspect-[4/3] overflow-hidden rounded-[14px] bg-rail">
              <ShapeTile kind={s.kind} />
              <span className="absolute top-3 left-3 font-mono text-[11px] text-muted-foreground">
                0{i + 1}
              </span>
            </div>
            <h3 className={cn(h3, "mt-5")}>{s.name}</h3>
            <p className="m-0 text-[15px] text-(--b-muted)">{s.use}</p>
          </div>
        ))}
      </div>
      <ul className={cn(rules, "mt-8 max-w-[76ch] text-[15.5px]")}>
        {shapeRules.map(r => (
          <li key={r}>{r}</li>
        ))}
      </ul>

      <div className="mt-16 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]">
        <div className={card}>
          <p className={lab}>Construction · dot field</p>
          <DotFieldConstruction />
          <p className={cap}>
            A regular grid, then a perspective: rows compress toward the horizon and the dot shrinks
            with them. The few lit points are the active units; the rest stay grey, so light means
            state.
          </p>
        </div>
        <SpecCard
          title="Proportions"
          rows={proportions}
          listClassName="[&>div]:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]"
        />
      </div>
    </>
  );
}

export function Shapes() {
  return (
    <Section
      id="shapes"
      n="01"
      file="shapes.svg"
      title={
        <>
          Six shapes, all from the <em>system</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          Points, lines and bars are what the visuals are made of; hairlines, pills and panels are
          the interface around them. Those six are the whole vocabulary: every piece is assembled
          from them, never illustrated.
        </p>
      }
    >
      <Vocabulary />

      <h3 className={cn(h3, "mt-16")}>Cropping a visual</h3>
      <div className="mt-4 grid gap-4 md:grid-cols-3">
        {crops.map(c => (
          <figure key={c.name} className="m-0 min-w-0">
            <div className="relative aspect-[4/3] overflow-hidden rounded-card border border-(--b-line)">
              <Visual name={c.name} x={c.x} y={c.y} />
              {c.x < 0.3 && (
                <div className="absolute inset-0 bg-[linear-gradient(90deg,color-mix(in_oklab,var(--color-rail)_60%,transparent)_0%,transparent_45%)]" />
              )}
            </div>
            <figcaption className={cap}>{c.cap}</figcaption>
          </figure>
        ))}
      </div>
      <ul className={cn(rules, "mt-6 max-w-[76ch] text-[15.5px]")}>
        {cropRules.map(r => (
          <li key={r}>{r}</li>
        ))}
      </ul>

      <h3 className={cn(h3, "mt-16")}>Never</h3>
      <div className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {donts.map(d => (
          <figure key={d.kind} className="m-0 min-w-0">
            <div className="relative aspect-[4/3] overflow-hidden rounded-card border border-dashed border-[color-mix(in_oklab,var(--color-destructive)_55%,transparent)]">
              <DontArt kind={d.kind} />
              <span className="absolute top-3 left-3 font-mono text-[12px] text-destructive">
                ✕
              </span>
            </div>
            <figcaption className={cap}>{d.cap}</figcaption>
          </figure>
        ))}
      </div>
    </Section>
  );
}
