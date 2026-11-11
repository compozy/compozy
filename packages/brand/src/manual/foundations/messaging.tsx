import { card, lab, voltFill } from "../cls";
import { Section } from "../parts/section";
import { ctas, messages } from "./data";

const proof: [figure: string, text: string][] = [
  ["26", "agent CLIs driven over ACP"],
  ["MIT", "open source, self-hosted, beta"],
  ["6", "companies implementing as design partners"],
  ["1", "Go binary and a SQLite daemon"],
  ["3,000+", "developers taught by the founder"],
  ["0", "telemetry, billing or cloud dependency"],
];

export function Messaging() {
  return (
    <Section
      id="messaging"
      n="10"
      file="messaging.md"
      title={
        <>
          What we say, <em>in order</em>
        </>
      }
      bg="ink2"
      status="approved"
      intro={
        <p>
          A ladder from positioning to sign-off. Every piece picks the rungs it needs, top to
          bottom, and never invents a new one without updating this page.
        </p>
      }
    >
      <ol className="m-0 grid list-none gap-0 overflow-hidden rounded-card border border-(--b-line) p-0">
        {messages.map((m, i) => (
          <li
            key={m.level}
            className="grid gap-3 border-b border-(--b-line) bg-(--b-card) px-6 py-6 last:border-0 md:grid-cols-[180px_minmax(0,1fr)_260px] md:gap-8"
          >
            <p className="m-0 mono-label text-(--b-accent)">
              {String(i + 1).padStart(2, "0")} · {m.level}
            </p>
            <p className="m-0 font-display text-[clamp(20px,2.2vw,28px)] leading-[1.2] tracking-[-0.02em]">
              {m.text}
            </p>
            <p className="m-0 text-[14px] text-(--b-muted)">{m.use}</p>
          </li>
        ))}
      </ol>
      <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
        <div className={card}>
          <p className={lab}>Calls to action</p>
          <ul className="m-0 mt-4 grid list-none gap-2 p-0">
            {ctas.map(([c, w]) => (
              <li key={c} className="flex items-center justify-between gap-3">
                <span className={`rounded-[10px] px-3.5 py-2 text-[14px] font-medium ${voltFill}`}>
                  {c}
                </span>
                <span className="text-[13px] text-(--b-muted)">{w}</span>
              </li>
            ))}
          </ul>
        </div>
        <div className={card}>
          <p className={lab}>Proof we can state (dated Nov 2026)</p>
          <ul className="m-0 mt-4 grid list-none gap-x-6 gap-y-3 p-0 text-[15px] sm:grid-cols-2">
            {proof.map(([f, t]) => (
              <li key={f}>
                <b className="font-display text-[22px]">{f}</b> {t}
              </li>
            ))}
          </ul>
        </div>
      </div>
    </Section>
  );
}
