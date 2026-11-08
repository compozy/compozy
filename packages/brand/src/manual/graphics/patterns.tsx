// 06 Patterns: the reusable patterns of the visual system (dot field, line field, lanes, bars),
// the hairline grid, how they are built, density rules.
import { cn } from "@compozy/ui/utils";
import type { CSSProperties, ReactNode } from "react";

import { Visual } from "../art/visual";
import type { VisualName } from "../art/visuals";
import { cap, card, h3, lab } from "../cls";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import { densities } from "./data";

const LINE = "var(--color-elevated)";
const hairlines = (step: number): CSSProperties => ({
  backgroundImage: `linear-gradient(${LINE} 1px,transparent 1px),linear-gradient(90deg,${LINE} 1px,transparent 1px)`,
  backgroundSize: `${step}px ${step}px`,
  backgroundPosition: "0 0",
});

const built: [string, string][] = [
  [
    "Dot field",
    "A regular grid of 2 px points, pitch 16 or 32 px, laid in perspective so rows compress toward the horizon. 1 in 10 points is lit in cyan; the lit ones form a path, never a shape.",
  ],
  [
    "Line field",
    "Contour lines, 1 px, #262626 to #383838, 12 to 24 px apart, bent by one smooth height map. A single ridge is lit, so the eye follows one active path.",
  ],
  [
    "Lanes",
    "One lane per agent, one 6 px bar per run, lanes 24 px apart. A 1 px now line sits at the right edge; runs still going carry the gradient, finished ones stay grey.",
  ],
  [
    "Bars",
    "Activity across a day in 2 px bars, 8 px apart, heights from real counts. The live window is the only part in gradient; the axis is mono 11 px.",
  ],
];

const swatches: { name: VisualName; title: string; text: string }[] = [
  {
    name: "field",
    title: "Dot field.",
    text: "The field visual: a large regular fleet of points, a few lit. Many sessions under one plane.",
  },
  {
    name: "terrain",
    title: "Line field.",
    text: "The terrain visual: contour lines with one lit ridge. The work surface and the active path across it.",
  },
  {
    name: "lanes",
    title: "Lanes.",
    text: "Live. One lane per agent, one bar per run, the now line at the edge. Drawn in code from run data.",
  },
  {
    name: "bars",
    title: "Bars.",
    text: "Live. Activity across a day in thin bars; the live window in gradient.",
  },
];

const tile = "relative aspect-[4/3] overflow-hidden rounded-card border border-(--b-line)";
const display =
  "m-0 max-w-[16ch] font-display text-[clamp(28px,3vw,40px)] leading-[1.05] font-normal tracking-[-0.03em]";
const kicker = "m-0 font-mono text-[12px] tracking-[0.12em] text-brand-cyan uppercase";

function Swatch({ title, text, children }: { title: string; text: string; children: ReactNode }) {
  return (
    <figure className="m-0 min-w-0">
      {children}
      <figcaption className={cap}>
        <b className="text-(--b-fg)">{title}</b> {text}
      </figcaption>
    </figure>
  );
}

function Families() {
  return (
    <div className="grid gap-4 lg:grid-cols-3">
      <Swatch
        title="Hairline grid."
        text="32 px on screen (8 px rhythm × 4), lines one step above the ground. One run of cells may carry light."
      >
        <div className={cn(tile, "bg-rail")} style={hairlines(32)}>
          <div className="absolute top-[97px] left-[129px] h-[31px] w-[95px] opacity-90 [background:var(--gradient-brand)]" />
          <span className="absolute bottom-4 left-4 font-mono text-[11px] text-muted-foreground">
            grid 32 px · #1b1b1b · 1 px
          </span>
        </div>
      </Swatch>
      {swatches.map(s => (
        <Swatch key={s.name} title={s.title} text={s.text}>
          <div className={tile}>
            <Visual name={s.name} />
          </div>
        </Swatch>
      ))}
      <div className={card}>
        <p className={lab}>How they are built</p>
        <dl className="m-0 mt-4 grid gap-3 text-[13.5px]">
          {built.map(([k, v]) => (
            <div key={k}>
              <dt className="font-medium">{k}</dt>
              <dd className="m-0 text-(--b-muted)">{v}</dd>
            </div>
          ))}
        </dl>
      </div>
    </div>
  );
}

function AsBackground() {
  return (
    <div className="mt-4 grid gap-4 lg:grid-cols-2">
      <div className="relative isolate min-h-[340px] overflow-hidden rounded-card border border-(--b-line)">
        <Visual name="field" x={0.8} y={0.6} className="-z-10" />
        <div className="absolute inset-0 -z-10 bg-[linear-gradient(90deg,var(--color-rail)_0%,color-mix(in_oklab,var(--color-rail)_85%,transparent)_32%,transparent_62%)]" />
        <div className="grid h-full content-end gap-3 p-8">
          <p className={kicker}>sessions</p>
          <p className={display}>Every run, under one plane.</p>
        </div>
      </div>
      <div className="relative isolate min-h-[340px] overflow-hidden rounded-card border border-(--b-line)">
        <Visual name="terrain" x={0.85} y={0.8} className="-z-10" />
        <div className="absolute inset-0 -z-10 bg-[linear-gradient(180deg,var(--color-rail)_0%,color-mix(in_oklab,var(--color-rail)_80%,transparent)_35%,transparent_75%)]" />
        <div className="grid h-full content-start gap-3 p-8">
          <p className={kicker}>runs</p>
          <p className={display}>It stops when the check passes.</p>
        </div>
      </div>
    </div>
  );
}

function DensityArt({ i }: { i: number }) {
  if (i === 0)
    return (
      <>
        <div className="absolute inset-0" style={hairlines(24)} />
        <div className="absolute inset-0 [mask-image:radial-gradient(circle_at_100%_100%,black_0%,transparent_38%)]">
          <Visual name="field" x={0.6} y={0.7} />
        </div>
      </>
    );
  if (i === 1) return <Visual name="converge" x={0.85} y={0.5} />;
  return <Visual name="swarm" x={0.6} y={0.5} />;
}

export function Patterns() {
  return (
    <Section
      id="patterns"
      n="06"
      file="patterns.css"
      title={
        <>
          Backgrounds that <em>stay back</em>
        </>
      }
      bg="black"
      status="proposal"
      intro={
        <p>
          Five patterns fill a surface without competing with what sits on it: the hairline grid,
          the dot field and the line field (Atmosphere), and lanes and bars (Live). Each has a
          density ceiling, and type always gets a quiet area.
        </p>
      }
    >
      <Families />

      <h3 className={cn(h3, "mt-16")}>As a background</h3>
      <AsBackground />

      <h3 className={cn(h3, "mt-16")}>Density</h3>
      <div className="mt-4 grid gap-4 md:grid-cols-3">
        {densities.map((d, i) => (
          <div key={d.name} className={card}>
            <div className="relative aspect-[16/9] overflow-hidden rounded-[12px] bg-rail">
              <DensityArt i={i} />
            </div>
            <div className="mt-4 flex items-baseline justify-between">
              <span className="font-display text-[22px] font-normal">{d.name}</span>
              <span className="font-mono text-[12px] text-brand-cyan">{d.cover} coverage</span>
            </div>
            <p className="m-0 mt-2 text-[14.5px] text-(--b-muted)">{d.use}</p>
            <p className="m-0 mt-2 text-[14.5px]">{d.rule}</p>
          </div>
        ))}
      </div>
      <DoDont
        className="mt-10"
        doContent={
          <p className="m-0 text-[15px] text-(--b-muted)">
            One pattern per surface. Product screens use the grid at most; marketing uses one
            Atmosphere pattern, with type on the quiet side.
          </p>
        }
        dontContent={
          <p className="m-0 text-[15px] text-(--b-muted)">
            A dot field plus lanes plus a grid on one surface, a pattern behind body text, or a
            pattern inside a product window.
          </p>
        }
      />
    </Section>
  );
}
