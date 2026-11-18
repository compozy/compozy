import { card, lab } from "../cls";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import { banned, glossary, voiceAttrs } from "./data";

const quote = "m-0 font-display text-[20px] leading-[1.35] tracking-[-0.01em]";

export function Voice() {
  return (
    <Section
      id="voice"
      n="09"
      file="voice.md"
      title={
        <>
          Calm, not cute. <em>Plain</em>, not vague.
        </>
      }
      status="approved"
      intro={
        <p>
          Lead with the outcome, then the mechanism, then the proof. The product is the subject of
          the sentence; “we” stays out of marketing copy and “you” lives in the interface.
        </p>
      }
    >
      <div className="grid gap-10">
        {voiceAttrs.map((a, i) => (
          <div key={a.name} className="grid gap-4 lg:grid-cols-[260px_minmax(0,1fr)]">
            <div>
              <p className="m-0 font-mono text-[12px] text-(--b-accent)">0{i + 1}</p>
              <p className="m-0 mt-2 font-display text-[34px] tracking-[-0.025em]">{a.name}</p>
              <p className="m-0 mt-2 text-[15px]">{a.is}</p>
              <p className="m-0 mt-1 text-[14px] text-(--b-muted)">Not: {a.isnt}</p>
            </div>
            <DoDont
              doContent={<p className={quote}>“{a.do}”</p>}
              dontContent={
                <p className={`${quote} text-(--b-muted) line-through decoration-destructive/60`}>
                  “{a.dont}”
                </p>
              }
            />
          </div>
        ))}
      </div>
      <div className="mt-16 grid gap-4 lg:grid-cols-2">
        <div className={card}>
          <p className={lab}>Words we don’t use</p>
          <dl className="m-0 mt-4 grid gap-0">
            {banned.map(([w, why]) => (
              <div
                key={w}
                className="grid grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)] gap-4 border-b border-(--b-line) py-2.5 last:border-0"
              >
                <dt className="font-mono text-[13px] text-destructive">{w}</dt>
                <dd className="m-0 text-[14px] text-(--b-muted)">{why}</dd>
              </div>
            ))}
          </dl>
        </div>
        <div className={card}>
          <p className={lab}>Glossary</p>
          <dl className="m-0 mt-4 grid gap-0">
            {glossary.map(([t, d]) => (
              <div
                key={`${t}-${d}`}
                className="grid grid-cols-[140px_minmax(0,1fr)] gap-4 border-b border-(--b-line) py-2.5 last:border-0"
              >
                <dt className="text-[14.5px] font-semibold">{t}</dt>
                <dd className="m-0 text-[14px] text-(--b-muted)">{d}</dd>
              </div>
            ))}
          </dl>
        </div>
      </div>
      <div className="mt-4 grid gap-4 md:grid-cols-3">
        <div className={card}>
          <p className={lab}>Interface</p>
          <ul className="m-0 mt-3 list-none space-y-1.5 p-0 font-mono text-[13px]">
            <li>Nothing delegated yet.</li>
            <li>Waiting for you</li>
            <li>PR status unknown</li>
            <li>Move to Later</li>
            <li>Couldn’t link 3 items: token expired</li>
          </ul>
        </div>
        <div className={card}>
          <p className={lab}>Rhythm</p>
          <p className="m-0 mt-3 text-[15px]">
            Name the reader’s problem. State the capability. Prove it with a mechanism, a command or
            an artifact. Three sentences, often fewer.
          </p>
        </div>
        <div className={card}>
          <p className={lab}>Numbers</p>
          <p className="m-0 mt-3 text-[15px]">
            Date them (“2.7k stars, Nov 2026”). Only claim users you can name. Design partners are
            never customers, and there is no price in public copy.
          </p>
        </div>
      </div>
    </Section>
  );
}
