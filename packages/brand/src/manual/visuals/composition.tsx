import { Visual } from "../art/visual";
import { card, specs } from "../cls";
import { Section } from "../parts/section";

/** ink scrim: rail at 100% → 85% → transparent toward the art */
const scrimSide =
  "bg-[linear-gradient(90deg,var(--color-rail)_0%,color-mix(in_oklab,var(--color-rail)_85%,transparent)_40%,transparent_72%)]";
const scrimBottom =
  "bg-[linear-gradient(0deg,var(--color-rail)_0%,color-mix(in_oklab,var(--color-rail)_85%,transparent)_32%,transparent_60%)]";

const compositionSpecs: [string, string][] = [
  ["Margin", "6.25% of the short side (100 px on 1600 × 900)"],
  ["Scrim", "#0a0a0a at 100% → 85% at 40% → 0 at 72% of the axis toward the art"],
  ["Headline", "Instrument Sans 400, −2.5%, max 12 em wide, 4–8 words"],
  ["Focal point", "x 0.75–0.9 landscape; y 0.25–0.45 portrait; Live visuals take x/y the same way"],
  ["Logo", "Top-left lockup or bottom-right symbol, never over the lit area"],
];

export function Composition() {
  return (
    <Section
      id="composition"
      n="07"
      file="composition.md"
      title={
        <>
          Art on one side, words on the <em>other</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          Every layout splits the canvas: the visual pushed to one side with its focal point, the
          text on the opposite side over a scrim. The visual is never centred behind a headline.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
        <figure className="m-0 min-w-0">
          <div className="relative isolate aspect-[16/9] overflow-hidden rounded-card border border-(--b-line)">
            <Visual name="swarm" x={0.85} className="-z-10" />
            <div className={`absolute inset-0 -z-10 ${scrimSide}`} />
            <div className="absolute inset-[6%] flex flex-col justify-end">
              <p className="m-0 mono-label text-[clamp(9px,.9vw,12px)] tracking-[0.14em] text-(--color-muted-foreground)">
                Control plane · always-on
              </p>
              <p className="m-0 mt-[2%] max-w-[12em] font-display text-[clamp(24px,3.6vw,52px)] leading-[1.05] tracking-[-0.025em]">
                The autonomous control plane for your agents.
              </p>
            </div>
            <div className="pointer-events-none absolute inset-y-0 left-[44%] border-l border-dashed border-[color-mix(in_oklab,var(--color-brand-cyan)_50%,transparent)]" />
            <span className="absolute top-3 left-[45%] font-mono text-[11px] text-(--color-brand-cyan)">
              scrim ends · 40–72%
            </span>
          </div>
          <figcaption className="mt-3 text-[15px] text-(--b-muted)">
            Landscape: art right (x 0.75–0.9), text left, bottom-aligned.
          </figcaption>
        </figure>
        <figure className="m-0 min-w-0">
          <div className="relative isolate mx-auto aspect-[4/5] max-w-[340px] overflow-hidden rounded-card border border-(--b-line)">
            <Visual name="converge" className="-z-10" />
            <div className={`absolute inset-0 -z-10 ${scrimBottom}`} />
            <div className="absolute inset-[8%] flex flex-col justify-end">
              <p className="m-0 font-display text-[clamp(20px,2.4vw,30px)] leading-[1.05] tracking-[-0.025em]">
                Every agent, one queue.
              </p>
            </div>
          </div>
          <figcaption className="mt-3 text-center text-[15px] text-(--b-muted)">
            Portrait: art top, text bottom. The portrait crop loads by itself.
          </figcaption>
        </figure>
      </div>
      <div className={`${card} mt-4`}>
        <dl className={specs}>
          {compositionSpecs.map(([dt, dd]) => (
            <div key={dt}>
              <dt>{dt}</dt>
              <dd>{dd}</dd>
            </div>
          ))}
        </dl>
      </div>
    </Section>
  );
}
