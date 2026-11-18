import { card, lab } from "../cls";
import { Section } from "../parts/section";
import { typeScale } from "./data";

const faces = [
  {
    name: "Instrument Sans",
    cls: "font-display",
    role: "Wordmark, headlines, numbers that matter",
    weights: "400 · 500",
    note: "Light in weight, precise in shape, so it sits on particles, light and fine lines without shouting. Headlines 400, wordmark and small headings 500. Never 600+.",
  },
  {
    name: "Figtree",
    cls: "font-sans",
    role: "Body, UI, long reading",
    weights: "400 · 500 · 600",
    note: "Chosen for its distinct Il1 and its legibility at 13–14 px in dense product UI.",
  },
  {
    name: "Geist Mono",
    cls: "font-mono",
    role: "Labels, code, terminal, data, the OS chip",
    weights: "400 · 500",
    note: "Uppercase with +12% tracking for labels; normal case for code. Never for headlines.",
  },
];

export function Type() {
  return (
    <Section
      id="type"
      n="06"
      file="type.css"
      title={
        <>
          Three voices, <em>one system</em>
        </>
      }
      bg="ink2"
      status="approved"
      intro={
        <p>
          Instrument Sans speaks, Figtree explains, Geist Mono reports. All three are open source
          (OFL) and self-hosted from <code>@fontsource-variable</code>.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-3">
        {faces.map(f => (
          <div key={f.name} className={`${card} flex flex-col`}>
            <p className={lab}>{f.role}</p>
            <p className={`${f.cls} m-0 mt-6 text-[88px] leading-none tracking-[-0.03em]`}>Aa</p>
            <p className={`${f.cls} m-0 mt-5 text-[26px] tracking-[-0.01em]`}>{f.name}</p>
            <p className={`${f.cls} m-0 mt-3 break-all text-[15px] text-(--b-muted)`}>
              ABCDEFGHIJKLMNOPQRSTUVWXYZ abcdefghijklmnopqrstuvwxyz 0123456789 Il1 O0 → ● ✓
            </p>
            <p className="m-0 mt-auto pt-6 font-mono text-[12px] text-(--b-muted)">
              Weights {f.weights}
            </p>
            <p className="m-0 mt-2 text-[14px] text-(--b-muted)">{f.note}</p>
          </div>
        ))}
      </div>
      <div className={`${card} mt-4 overflow-hidden p-0! sm:p-0!`}>
        {typeScale.map(([n, cls, spec, sample]) => (
          <div
            key={n}
            className="grid items-baseline gap-2 border-b border-(--b-line) px-6 py-6 last:border-0 md:grid-cols-[200px_minmax(0,1fr)] md:gap-8"
          >
            <div>
              <p className="m-0 text-[15px] font-medium">{n}</p>
              <p className="m-0 font-mono text-[11.5px] text-(--b-muted)">{spec}</p>
            </div>
            <p className={`${cls} m-0 min-w-0 pb-1`}>{sample}</p>
          </div>
        ))}
      </div>
      <div className="mt-4 grid gap-4 md:grid-cols-3">
        <div className={card}>
          <p className={lab}>Emphasis</p>
          <p className="m-0 mt-3 font-display text-[30px] leading-tight tracking-[-0.02em]">
            Agents that keep <em>running</em>.
          </p>
          <p className="m-0 mt-3 text-[14px] text-(--b-muted)">
            One word per headline can carry the gradient. Never two, never in body text.
          </p>
        </div>
        <div className={card}>
          <p className={lab}>Numbers</p>
          <p className="m-0 mt-3 font-display text-[64px] leading-none tracking-[-0.035em]">26</p>
          <p className="m-0 mt-2 mono-label tracking-[0.12em] text-(--b-muted)">
            agent CLIs over ACP
          </p>
          <p className="m-0 mt-3 text-[14px] text-(--b-muted)">
            Big numbers in Instrument Sans, unit in mono. Date every number in public copy.
          </p>
        </div>
        <div className={card}>
          <p className={lab}>Case</p>
          <p className="m-0 mt-3 text-[16px]">
            Sentence case for every heading and label. Uppercase only in mono metadata:{" "}
            <span className="mono-label tracking-[0.12em] text-(--b-accent)">
              release 0.6 · beta
            </span>
            .
          </p>
        </div>
      </div>
    </Section>
  );
}
