import type { CSSProperties } from "react";

import { card, h3, lab } from "../cls";
import { Section } from "../parts/section";
import { essence, values } from "./data";

export function Essence() {
  return (
    <Section
      id="essence"
      n="01"
      file="essence.md"
      title={
        <>
          The autonomous control plane for your <em>agents</em>
        </>
      }
      status="approved"
      intro={
        <p>
          A harness runs the model. Compozy is where you see and command all of them, and where they
          keep working on their own until they need you. Everything in this manual serves that one
          idea: <strong>many agents, one autonomous control plane, shown through our UI</strong>.
        </p>
      }
    >
      <dl className="m-0 grid gap-px overflow-hidden rounded-card border border-(--b-line) bg-(--b-line) md:grid-cols-2">
        {essence.map(([k, v]) => (
          <div key={k} className="bg-(--b-card) p-6 sm:p-7">
            <dt className={lab}>{k}</dt>
            <dd className="m-0 mt-2.5 font-display text-[22px] leading-[1.25] tracking-[-0.015em]">
              {v}
            </dd>
          </div>
        ))}
      </dl>
      <h3 className={`${h3} mt-16`}>Five values, each a product behaviour</h3>
      <div className="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        {values.map(([t, d], i) => (
          <div
            key={t}
            className={card}
            data-reveal
            style={{ "--d": `${i * 60}ms` } as CSSProperties}
          >
            <p className="m-0 font-mono text-[12px] text-(--b-accent)">0{i + 1}</p>
            <p className="m-0 mt-6 font-display text-[21px] leading-[1.15] tracking-[-0.015em]">
              {t}
            </p>
            <p className="m-0 mt-3 text-[15px] text-(--b-muted)">{d}</p>
          </div>
        ))}
      </div>
    </Section>
  );
}
