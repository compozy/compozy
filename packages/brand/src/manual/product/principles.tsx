// 01 Principles: how the brand behaves inside a tool people watch all day.
import { card, lab } from "../cls";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import { loud, principles, sessions } from "./data";
import { SessionRow } from "./session-row";

const list = "grid gap-0.5 rounded-[12px] border border-line bg-(--color-rail) p-2";

export function Principles() {
  return (
    <Section
      id="principles"
      n="01"
      file="principles.md"
      title={
        <>
          One desktop, many agents, calm until <em>needs you</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          Compozy is a control plane for your agents, and its desktop is the product. Inside it the
          brand steps back: ink, hairlines, faces and type, so the one agent that needs a person
          stands out.
        </p>
      }
    >
      <div className="grid gap-4 md:grid-cols-2">
        {principles.map(p => (
          <div key={p.n} className={card}>
            <p className={lab}>
              <span className="text-(--b-accent)">{p.n}</span> · principle
            </p>
            <h3 className="mt-4 mb-3 font-display text-[28px] leading-[1.1] tracking-[-0.02em]">
              {p.title}
            </h3>
            <p className="m-0 text-[15.5px] text-(--b-muted)">{p.body}</p>
            <p className="m-0 mt-5 border-t border-(--b-line) pt-4 font-mono text-[12.5px] leading-[1.6] text-(--b-fg)">
              {p.rule}
            </p>
          </div>
        ))}
      </div>
      <DoDont
        className="mt-10"
        doLabel="Do · one loud thing"
        dontLabel="Don't · everything loud"
        doContent={
          <div className={list}>
            {sessions.slice(0, 6).map(s => (
              <SessionRow key={s.name} {...s} />
            ))}
          </div>
        }
        dontContent={
          <div className={list}>
            {sessions.slice(0, 6).map((s, i) => {
              const c = loud[i] ?? loud[0]!;
              return (
                <div
                  key={s.name}
                  className="flex min-h-8 items-center justify-between rounded-[6px] px-2.5 text-[13.5px] font-semibold text-(--color-fg-strong)"
                  style={{
                    background: `color-mix(in oklab, ${c} 20%, transparent)`,
                    borderLeft: `3px solid ${c}`,
                  }}
                >
                  <span>{s.name}</span>
                  <span className="rounded-full px-2 text-[11px]" style={{ background: c }}>
                    {s.state.toUpperCase()}
                  </span>
                </div>
              );
            })}
          </div>
        }
      />
    </Section>
  );
}
