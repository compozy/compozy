import { cn } from "@compozy/ui/utils";

import { card, h3, lab, rules } from "../cls";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import { modes, nevers, rewrites } from "./data";

const quote = "m-0 font-display text-[22px] leading-[1.3] tracking-[-0.015em]";

function Modes() {
  return (
    <div className="grid gap-4 lg:grid-cols-2">
      {modes.map(m => (
        <article key={m.id} className={cn(card, "flex flex-col gap-4")}>
          <div className="flex items-center justify-between">
            <p className={lab}>{m.id} mode</p>
            <span className="font-mono text-[12px] text-(--b-accent)">
              {m.templates.length} templates
            </span>
          </div>
          <h3 className={h3}>{m.title}</h3>
          <p className="m-0 text-[15.5px]">{m.goal}</p>
          <dl className="m-0 grid gap-3 text-[14.5px]">
            <div>
              <dt className={lab}>When</dt>
              <dd className="m-0 mt-1">{m.when}</dd>
            </div>
            <div>
              <dt className={lab}>Shows</dt>
              <dd className="m-0 mt-1">
                <ul className={rules}>
                  {m.shows.map(s => (
                    <li key={s}>{s}</li>
                  ))}
                </ul>
              </dd>
            </div>
            <div>
              <dt className={lab}>Never</dt>
              <dd className="m-0 mt-1 text-(--b-muted)">{m.never}</dd>
            </div>
          </dl>
          <p className="m-0 mt-auto rounded-[14px] bg-rail p-6 font-display text-[30px] leading-[1.05] tracking-[-0.03em] text-fg">
            {m.example}
          </p>
          <p className="m-0 font-mono text-[12px] text-(--b-muted)">{m.templates.join(" · ")}</p>
        </article>
      ))}
    </div>
  );
}

export function Principles() {
  return (
    <Section
      id="principles"
      n="01"
      file="principles.md"
      title={
        <>
          Two modes, <em>no</em> offer
        </>
      }
      status="proposal"
      intro={
        <p>
          Compozy has nothing to sell in an ad: no price, no plan, no trial. Every piece is either
          brand, which plants the name, or proof, which shows the mechanism working. Pick the mode
          before the first word.
        </p>
      }
    >
      <Modes />

      <h3 className={cn(h3, "mt-16")}>Never in an ad</h3>
      <p className="m-0 mb-5 max-w-[62ch] text-[15px] text-(--b-muted)">
        These come from the positioning rules, and they bind every format, every channel and every
        variant in a test.
      </p>
      <ol className="m-0 grid list-none gap-px overflow-hidden rounded-card border border-(--b-line) bg-(--b-line) p-0 md:grid-cols-2">
        {nevers.map(([r, why], i) => (
          <li key={r} className="grid grid-cols-[36px_minmax(0,1fr)] gap-3 bg-(--b-card) p-5">
            <span className="font-mono text-[12px] text-destructive">
              ✕ {String(i + 1).padStart(2, "0")}
            </span>
            <div>
              <p className="m-0 text-[15.5px] font-semibold">{r}</p>
              <p className="m-0 mt-1 text-[14px] text-(--b-muted)">{why}</p>
            </div>
          </li>
        ))}
      </ol>

      <div className="mt-10 grid gap-4">
        {rewrites.map(([d, o]) => (
          <DoDont
            key={o}
            doLabel="Compozy"
            dontLabel="Generic SaaS ad"
            doContent={<p className={quote}>“{o}”</p>}
            dontContent={
              <p
                className={cn(
                  quote,
                  "text-(--b-muted) line-through decoration-[color-mix(in_oklab,var(--color-destructive)_60%,transparent)]"
                )}
              >
                “{d}”
              </p>
            }
          />
        ))}
      </div>
    </Section>
  );
}
