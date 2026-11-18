import { card, h3, lab } from "../cls";
import { Section } from "../parts/section";
import { altPattern, consent, credits } from "./data";
import { SpecCard } from "../parts/spec-card";

function Checklist() {
  return (
    <div className={card}>
      <p className={lab}>Consent checklist</p>
      <ul className="m-0 mt-4 grid list-none gap-0 p-0">
        {consent.map(([k, v]) => (
          <li key={k} className="flex gap-4 border-b border-(--b-line) py-3 last:border-0">
            <span className="mt-0.5 grid size-5 shrink-0 place-items-center rounded-[5px] border border-(--b-accent) font-mono text-[11px] text-(--b-accent)">
              ✓
            </span>
            <span>
              <strong className="text-[15px]">{k}</strong>
              <span className="block text-[14.5px] text-(--b-muted)">{v}</span>
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function Consent() {
  return (
    <Section
      id="consent"
      n="07"
      file="consent.md"
      title={
        <>
          Permission, alt text, <em>credit</em>
        </>
      }
      intro={
        <p>
          Every person in a Compozy image agreed to be there, every image can be read by a screen
          reader, and every photographer is named. These three checks run before anything is
          published.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
        <Checklist />
        <div className="grid content-start gap-4">
          <div className={card}>
            <p className={lab}>Alt text pattern</p>
            <p className="m-0 mt-3 font-mono text-[13px] text-(--b-accent)">
              who · doing what · where · the one detail that matters
            </p>
            <p className="m-0 mt-3 text-[14.5px] text-(--b-muted)">
              One sentence, under 125 characters, no “image of”. Product shots name the screen and
              its state. Decorative Atmosphere visuals get <code>alt=""</code>.
            </p>
          </div>
          <SpecCard
            title="Credit lines"
            rows={credits}
            listClassName="mt-3"
            valueClassName="font-mono text-[13px]"
          />
        </div>
      </div>
      <h3 className={`${h3} mt-16`}>Alt text, before and after</h3>
      <div className="mt-4 grid gap-4 md:grid-cols-3">
        {altPattern.map(a => (
          <div key={a.bad} className={card}>
            <p className="m-0 font-mono text-[13px] text-destructive line-through decoration-destructive/60">
              {a.bad}
            </p>
            <p className="m-0 mt-4 text-[15.5px]">{a.good}</p>
          </div>
        ))}
      </div>
    </Section>
  );
}
