import { card, lab } from "../cls";
import { Section } from "../parts/section";
import { changelogRules, changelogSample as c } from "./data";
import { Rows } from "./rows";

function Item({ text }: { text: string }) {
  const m = /^(.*) \((#\d+)\)$/.exec(text);
  return (
    <li className="flex justify-between gap-4">
      <span>{m ? m[1] : text}</span>
      {m && <span className="shrink-0 font-mono text-[12.5px] text-(--b-muted)">{m[2]}</span>}
    </li>
  );
}

export function Changelog() {
  return (
    <Section
      id="changelog"
      n="04"
      file="CHANGELOG.md"
      title={
        <>
          Release notes that read like <em>work</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          The changelog is the most honest page Compozy has: what changed, when, and what is still
          missing. Every release follows the same shape so a reader can scan forty of them in a
          minute.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <article className="min-w-0 rounded-card border border-(--b-line) bg-(--b-card) p-7 sm:p-10">
          <div className="flex flex-wrap items-baseline gap-x-5 gap-y-1">
            <h3 className="m-0 font-display text-[56px] leading-none tracking-[-0.03em]">
              {c.version}
            </h3>
            <span className="mono-label text-[12.5px] text-(--b-muted)">{c.date}</span>
            <span className="rounded-md border border-dashed border-(--b-muted) px-2 py-0.5 mono-label text-[11px] text-(--b-muted)">
              sample
            </span>
          </div>
          <ul className="m-0 mt-6 grid list-none gap-2 p-0">
            {c.highlights.map(h => (
              <li key={h} className="font-display text-[22px] leading-[1.25] tracking-[-0.015em]">
                <span className="text-(--b-accent)">+ </span>
                {h}
              </li>
            ))}
          </ul>
          {c.sections.map(([s, items]) => (
            <div key={s} className="mt-7 border-t border-(--b-line) pt-5">
              <p className={lab}>{s}</p>
              <ul className="m-0 mt-3 grid list-none gap-2 p-0 text-[15px]">
                {items.map(it => (
                  <Item key={it} text={it} />
                ))}
              </ul>
            </div>
          ))}
          <div className="mt-7 rounded-[12px] border border-dashed border-[color-mix(in_oklab,var(--color-warning)_55%,transparent)] p-4">
            <p className="m-0 mono-label text-warning">Known issues</p>
            <p className="m-0 mt-2 text-[15px]">{c.known}</p>
          </div>
        </article>
        <div className="grid content-start gap-4">
          <div className={card}>
            <p className={lab}>Format</p>
            <Rows rows={changelogRules} className="mt-3" />
          </div>
          <p className="m-0 text-[14px] text-(--b-muted)">
            Sample release, illustrative content. The three highlights feed og-changelog word for
            word.
          </p>
        </div>
      </div>
    </Section>
  );
}
