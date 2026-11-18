import { cn } from "@compozy/ui/utils";

import { card, h3, lab } from "../cls";
import { Frame } from "../kit-frame";
import { Section } from "../parts/section";
import { hierarchy, steps } from "./data";

/** reading-order pins over ad-square-proof: step, x %, y % */
const pins: [string, number, number][] = [
  ["01", 7, 16],
  ["03", 7, 44],
  ["02", 7, 81],
  ["04", 93, 81],
];

function Anatomy() {
  return (
    <div className="mt-16 grid items-start gap-8 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]">
      <div className="relative">
        <Frame id="ad-square-proof" />
        {pins.map(([n, x, y]) => (
          <span
            key={n}
            className="absolute grid h-7 w-7 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full bg-(--color-brand-cyan) font-mono text-[11px] font-semibold text-rail shadow-[0_0_0_4px_color-mix(in_oklab,var(--color-rail)_60%,transparent)]"
            style={{ left: `${x}%`, top: `${y * 0.94}%` }}
          >
            {n}
          </span>
        ))}
      </div>
      <div>
        <p className={lab}>Anatomy · ad-square-proof</p>
        <ol className="m-0 mt-4 grid list-none gap-3 p-0 text-[15px]">
          <li>
            <b className="font-mono text-(--b-accent)">01</b> Hook, 8cqi (86 px at 1080), two lines,
            top left under the lockup.
          </li>
          <li>
            <b className="font-mono text-(--b-accent)">03</b> Proof: our UI in a framed panel, a
            Live visual (here the approval dock) at the full width of the margin box.
          </li>
          <li>
            <b className="font-mono text-(--b-accent)">02</b> Mechanism, 2.6cqi, muted, one
            sentence, bottom left.
          </li>
          <li>
            <b className="font-mono text-(--b-accent)">04</b> CTA, volt, bottom right, the last
            thing the eye reaches.
          </li>
        </ol>
        <dl className="m-0 mt-8 grid gap-0">
          {hierarchy.map(([t, d]) => (
            <div
              key={t}
              className="grid gap-1 border-b border-(--b-line) py-3 md:grid-cols-[150px_minmax(0,1fr)] md:gap-6"
            >
              <dt className="text-[14.5px] font-semibold">{t}</dt>
              <dd className="m-0 text-[14.5px] text-(--b-muted)">{d}</dd>
            </div>
          ))}
        </dl>
      </div>
    </div>
  );
}

export function Structure() {
  return (
    <Section
      id="structure"
      n="02"
      file="structure.md"
      title={
        <>
          Hook, mechanism, proof, <em>CTA</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          A feed gives an ad about six seconds. The order never changes: the reader’s problem, how
          Compozy handles it, something they can check in the product, one next step. Brand mode
          swaps the proof for an Atmosphere visual.
        </p>
      }
    >
      <ol className="m-0 grid list-none gap-4 p-0 sm:grid-cols-2 lg:grid-cols-4">
        {steps.map(s => (
          <li key={s.n} className={cn(card, "flex flex-col gap-2")}>
            <div className="flex items-baseline justify-between">
              <span className="font-display text-[40px] leading-none tracking-[-0.04em] text-(--b-accent)">
                {s.n}
              </span>
              <span className={lab}>{s.time}</span>
            </div>
            <h3 className={cn(h3, "mt-2")}>{s.title}</h3>
            <p className="m-0 text-[14.5px]">{s.body}</p>
            <p className="m-0 mt-auto border-t border-(--b-line) pt-3 font-mono text-[12px] text-(--b-muted)">
              size vs hook · {s.size}
            </p>
          </li>
        ))}
      </ol>
      <Anatomy />
    </Section>
  );
}
