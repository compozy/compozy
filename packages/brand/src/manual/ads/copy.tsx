import { cn } from "@compozy/ui/utils";

import { card, h3, lab, voltFill } from "../cls";
import { Section } from "../parts/section";
import { copyBank } from "./data";

const words = (s: string) => s.replace(/[.,]/g, "").split(/\s+/).filter(Boolean).length;

const notes: [label: string, text: string][] = [
  [
    "Hook formula",
    "Name the outcome in the reader’s words (“one desktop”, “check in on their own”), or the problem they had last night (“five terminals”). Never the category.",
  ],
  [
    "Testing",
    "Change one thing per variant: the hook, or the visual, or the CTA. Two variants per ad set, 7 days minimum before a call.",
  ],
  [
    "Founder voice",
    "Posts promoted from Pedro’s account use first person and the 3,000-developers story. Brand accounts never say “I” or “we”.",
  ],
];

export function Copy() {
  return (
    <Section
      id="copy"
      n="04"
      file="copy-bank.md"
      title={
        <>
          Lines that are <em>ready</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          Approved lines per format, in the Compozy voice. Headlines stay at 6 words or fewer,
          bodies at one sentence, CTAs start with a verb. New lines get added here before they run,
          not after.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-2">
        {copyBank.map(b => (
          <article key={b.format} className={card}>
            <div className="flex flex-wrap items-baseline justify-between gap-2">
              <h3 className={h3}>{b.format}</h3>
              <span className="font-mono text-[12px] text-(--b-muted)">{b.ids}</span>
            </div>
            <p className={cn(lab, "mt-5")}>Headlines · ≤ 6 words</p>
            <ul className="m-0 mt-2 grid list-none gap-0 p-0">
              {b.headlines.map(h => (
                <li
                  key={h}
                  className="flex items-baseline justify-between gap-4 border-b border-(--b-line) py-2.5 last:border-0"
                >
                  <span className="font-display text-[21px] leading-[1.2] tracking-[-0.02em]">
                    {h}
                  </span>
                  <span className="shrink-0 font-mono text-[12px] text-(--b-accent)">
                    {words(h)}w
                  </span>
                </li>
              ))}
            </ul>
            <p className={cn(lab, "mt-5")}>Body lines</p>
            <ul className="m-0 mt-2 grid list-none gap-2 p-0 text-[15px] text-(--b-muted)">
              {b.bodies.map(x => (
                <li key={x}>{x}</li>
              ))}
            </ul>
            <p className={cn(lab, "mt-5")}>CTAs</p>
            <div className="mt-2 flex flex-wrap gap-2">
              {b.ctas.map(c => (
                <span
                  key={c}
                  className={cn("rounded-[10px] px-3.5 py-2 text-[14px] font-medium", voltFill)}
                >
                  {c}
                </span>
              ))}
            </div>
          </article>
        ))}
      </div>
      <div className="mt-4 grid gap-4 md:grid-cols-3">
        {notes.map(([label, text]) => (
          <div key={label} className={card}>
            <p className={lab}>{label}</p>
            <p className="m-0 mt-3 text-[15px]">{text}</p>
          </div>
        ))}
      </div>
    </Section>
  );
}
