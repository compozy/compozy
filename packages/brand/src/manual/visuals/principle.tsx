import { card, h3, lab, rules } from "../cls";
import { Visual } from "../art/visual";
import { Section } from "../parts/section";
import { principleFrames } from "./data";

export function Principle() {
  return (
    <Section
      id="principle"
      n="01"
      file="principle.md"
      title={
        <>
          Many agents, one <em>system</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          Compozy is the autonomous control plane for your agents. Every visual says that: a lot of
          independent work across harnesses, held by one precise control that runs on its own.
          Atmosphere says it as mood, with particles, light and fine lines. Live says it with our
          UI: the desktop, the always-on agents, the board and the handoffs.
        </p>
      }
    >
      <div className="grid gap-4 md:grid-cols-3">
        {principleFrames.map(([t, d, v]) => (
          <figure key={t} className="m-0 min-w-0">
            <div className="relative overflow-hidden rounded-card border border-(--b-line)">
              <Visual name={v} inline ratio="4/3" />
            </div>
            <figcaption className="mt-3">
              <span className={lab}>{t}</span>
              <span className="mt-1 block text-[15px] text-(--b-muted)">{d}</span>
            </figcaption>
          </figure>
        ))}
      </div>
      <div className="mt-10 grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
        <div className={card}>
          <p className={h3}>Why not a shape</p>
          <p className="m-0 text-[15px] text-(--b-muted)">
            The bottleneck stopped being the model; it became managing the sessions. A control plane
            is where you see and command every harness at once. Its best picture is the interface
            itself, so the product UI is the brand's main image, and Atmosphere sets the mood around
            it. The terminal is the before picture, not the brand.
          </p>
          <p className="m-0 mt-3 text-[15px] text-(--b-muted)">
            The symbol stays the only drawn shape of the brand. Nothing in this chapter imitates it,
            repeats it or turns it into a pattern.
          </p>
        </div>
        <div className={card}>
          <p className={lab}>Four materials, nothing else</p>
          <ul className={`${rules} mt-4 text-[15px]`}>
            <li>
              <strong>Points.</strong> Particles and dots are agents and units of work. Many of
              them, small, sharp.
            </li>
            <li>
              <strong>Light.</strong> Blue to cyan light marks what is active. Everything else stays
              white or grey on black.
            </li>
            <li>
              <strong>Hairlines.</strong> Lines are 1 px or thinner at any size: connectors,
              contours, scales.
            </li>
            <li>
              <strong>Black.</strong> Neutral black (#0a0a0a, rasters to #000) is the ground. At
              least half of every canvas stays empty.
            </li>
          </ul>
        </div>
      </div>
    </Section>
  );
}
