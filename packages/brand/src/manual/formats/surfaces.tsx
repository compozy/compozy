// Formats, surfaces: the three grounds every piece starts from, with tokens, contrast, the visual
// each carries and use. Volt is an accent, not a ground.
import { cn } from "@compozy/ui/utils";

import { Visual } from "../art/visual";
import type { VisualName } from "../art/visuals";
import { cap, card, lab } from "../cls";
import { grade, ratio } from "../contrast";
import { Section } from "../parts/section";
import { withOffsets } from "../text-runs";
import { surfaces, voltAccent, type SurfaceRecipe } from "./data";

const visualOf: Record<SurfaceRecipe["id"], { name: VisualName; x: number; y: number }> = {
  ink: { name: "swarm", x: 0.86, y: 0.5 },
  black: { name: "field", x: 0.88, y: 0.6 },
  paper: { name: "lanes", x: 0.5, y: 0.5 },
};

function ShareBar() {
  return (
    <>
      <div
        className="flex h-12 overflow-hidden rounded-[12px] shadow-[0_0_0_1px_var(--b-line)]"
        role="img"
        aria-label="Share of surfaces: ink 70 %, paper 20 %, black 10 %"
      >
        {surfaces.map(s => (
          <div
            key={s.id}
            className="grid min-w-0 place-items-center font-mono text-[12px]"
            style={{ flex: parseFloat(s.share), background: s.fill.bg, color: s.fill.fg }}
          >
            {parseFloat(s.share) >= 8 ? `${s.name} ${s.share}` : ""}
          </div>
        ))}
      </div>
      <p className={cap}>Share of pieces across a quarter: ink 70 %, paper 20 %, black 10 %.</p>
    </>
  );
}

function SurfaceCard({ s }: { s: SurfaceRecipe }) {
  const fg = ratio(s.fg, s.bg);
  const ac = ratio(s.accent, s.bg);
  const v = visualOf[s.id];
  return (
    <article className={cn(card, "overflow-hidden p-0!")}>
      <div
        className="relative aspect-[16/9] [container-type:inline-size]"
        style={{ background: s.fill.bg, color: s.fill.fg }}
      >
        <Visual name={v.name} x={v.x} y={v.y} theme={s.id === "paper" ? "light" : "dark"} />
        <div
          className="absolute inset-0"
          style={{
            background: `linear-gradient(90deg, ${s.fill.bg} 0%, color-mix(in oklab, ${s.fill.bg} 85%, transparent) 36%, transparent 70%)`,
          }}
        />
        <div className="absolute inset-[6.25cqi] flex flex-col justify-end">
          <p
            className="m-0 mono-label"
            style={{ color: s.fill.accent, fontSize: "2.4cqi", letterSpacing: "0.12em" }}
          >
            {s.name} · {s.share}
          </p>
          <p className="m-0 mt-[1.4cqi] max-w-[7em] font-display text-[6.4cqi] leading-[1.05] font-normal tracking-[-0.03em]">
            Goal, act, verify, stop.
          </p>
          <p className="m-0 mt-[1.8cqi] font-mono text-[2cqi]" style={{ color: s.fill.muted }}>
            compozy loop start review-pr
          </p>
        </div>
      </div>
      <div className="grid gap-5 p-[22px] sm:p-7">
        <div className="grid grid-cols-2 gap-2">
          {s.tokens.map(([k, hex, fill]) => (
            <div key={k} className="min-w-0">
              <span
                className="block h-8 rounded-[8px] shadow-[0_0_0_1px_var(--b-line)]"
                style={{ background: fill }}
              />
              <span className="mt-1.5 block truncate font-mono text-[11px] text-(--b-muted)">
                {k}
              </span>
              <span className="block truncate font-mono text-[11.5px]">{hex}</span>
            </div>
          ))}
        </div>
        <dl className="m-0 grid grid-cols-2 gap-3 font-mono text-[12.5px]">
          <div>
            <dt className={lab}>Text</dt>
            <dd className="m-0 mt-1">
              {fg.toFixed(1)}:1 · {grade(fg)}
            </dd>
          </div>
          <div>
            <dt className={lab}>Accent</dt>
            <dd className="m-0 mt-1">
              {ac.toFixed(1)}:1 · {grade(ac)}
            </dd>
          </div>
        </dl>
        <div className="grid gap-2 text-[14px]">
          <p className="m-0">
            <span className="font-mono text-[12px] text-(--b-accent)">USE</span>{" "}
            <span className="text-(--b-muted)">{s.when}</span>
          </p>
          <p className="m-0">
            <span className="font-mono text-[12px] text-destructive">AVOID</span>{" "}
            <span className="text-(--b-muted)">{s.avoid}</span>
          </p>
        </div>
      </div>
    </article>
  );
}

function VoltCard() {
  return (
    <div className={card}>
      <p className={lab}>Volt, the accent</p>
      <div className="mt-4 flex items-center gap-4">
        <span className="grid h-16 w-16 shrink-0 place-items-center rounded-[14px] bg-brand-volt font-mono text-[11px] text-brand-ink">
          volt
        </span>
        <p className="m-0 font-mono text-[13px]">{voltAccent.hex} · white text 5.2:1</p>
      </div>
      <p className="m-0 mt-4 text-[14.5px] text-(--b-muted)">
        <span className="font-mono text-[12px] text-(--b-accent)">USE</span> {voltAccent.use}
      </p>
      <p className="m-0 mt-2 text-[14.5px] text-(--b-muted)">
        <span className="font-mono text-[12px] text-destructive">AVOID</span> {voltAccent.avoid}{" "}
        Former volt and gradient pieces are ink with a visual.
      </p>
    </div>
  );
}

const C = "text-brand-cyan";
const S = "text-warning";
const M = "text-muted-foreground";

/** The recipe as [text, tone] runs, so the formatter can never reflow the whitespace. */
const RECIPE: [text: string, tone?: string][] = [
  ["/* one class per surface; the visual reads the same name */", M],
  [
    "\n.surface-ink   { background: var(--color-rail); color: var(--color-fg);   --accent: var(--color-brand-cyan); }" +
      "\n.surface-black { background: #000;              color: var(--color-fg);   --accent: var(--color-brand-cyan); }" +
      "\n.surface-paper { background: #f6f6f6;           color: var(--color-rail); --accent: #2448E6; }\n\n",
  ],
  ["<Visual", C],
  [" name="],
  ['"swarm"', S],
  [" x={0.86} "],
  ["/>", C],
  ["                 "],
  ["// Atmosphere on ink or black", M],
  ["\n"],
  ["<Visual", C],
  [" name="],
  ['"lanes"', S],
  [" theme="],
  ['"light"', S],
  [" "],
  ["/>", C],
  ["       "],
  ["// Live on paper", M],
];

const RECIPE_RUNS = withOffsets(RECIPE);

function Recipe() {
  return (
    <div className={card}>
      <p className={lab}>Recipe in code</p>
      <pre className="m-0 mt-4 overflow-x-auto rounded-[12px] bg-rail p-5 font-mono text-[13px] leading-[1.7] text-fg">
        {RECIPE_RUNS.map(({ at, run: [text, tone] }) =>
          tone ? (
            <span key={at} className={tone}>
              {text}
            </span>
          ) : (
            text
          )
        )}
      </pre>
    </div>
  );
}

export function Surfaces() {
  return (
    <Section
      id="surfaces"
      n="05"
      file="surfaces.css"
      title={
        <>
          Three grounds, one <em>accent</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          Every piece starts on ink, black or paper. The surface decides the text colours, the
          accent and which visual it can carry: Atmosphere on the dark grounds, Live on dark or
          paper. Volt is not a ground; it is the accent on top.
        </p>
      }
    >
      <ShareBar />
      <div className="mt-8 grid gap-4 md:grid-cols-3">
        {surfaces.map(s => (
          <SurfaceCard key={s.id} s={s} />
        ))}
      </div>
      <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)]">
        <VoltCard />
        <Recipe />
      </div>
    </Section>
  );
}
