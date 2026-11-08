import { cn } from "@compozy/ui/utils";

import { Visual } from "../art/visual";
import { lab } from "../cls";
import { Section } from "../parts/section";
import { pairingLevels } from "./data";

const scrim =
  "bg-[linear-gradient(90deg,var(--color-rail)_0%,color-mix(in_oklab,var(--color-rail)_85%,transparent)_42%,transparent_75%)]";

export function Pairing() {
  return (
    <Section
      id="pairing"
      n="08"
      file="pairing.md"
      title={
        <>
          Three volumes, one <em>voice</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          The same headline style works at every volume because the type stays light. What changes
          is how much of the canvas the visual takes and which family it comes from.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-3">
        {pairingLevels.map((l, i) => (
          <figure key={l.title} className="m-0 min-w-0">
            <div className="relative isolate aspect-[1200/630] overflow-hidden rounded-card border border-(--b-line)">
              <Visual name={l.visual} x={l.x} className={cn("-z-10", i === 2 && "opacity-60")} />
              <div className={`absolute inset-0 -z-10 ${scrim}`} />
              <img
                src="/brand/svg/compozy-lockup-dark.svg"
                alt=""
                className="absolute top-[8%] left-[6%] h-[7%] w-auto"
              />
              <p className="absolute bottom-[10%] left-[6%] m-0 max-w-[60%] font-display text-[clamp(16px,1.9vw,26px)] leading-[1.06] tracking-[-0.025em]">
                {l.headline}
              </p>
            </div>
            <figcaption className="mt-3">
              <span className={lab}>{l.title}</span>
              <span className="mt-1 block text-[15px] text-(--b-muted)">{l.note}</span>
            </figcaption>
          </figure>
        ))}
      </div>
    </Section>
  );
}
