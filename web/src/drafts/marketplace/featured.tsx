/** Featured comes from the catalog's own curation; the art only frames it. */
import { cn } from "@compozy/ui";

import plateBranch from "../landing/assets/plate-branch.jpg";
import plateFinal from "../landing/assets/plate-final.jpg";
import plateGates from "../landing/assets/plate-gates.jpg";
import plateOrder from "../landing/assets/plate-order.jpg";
import platePulses from "../landing/assets/plate-pulses.jpg";
import plateTerrain from "../landing/assets/plate-terrain.jpg";
import ucSpec from "../landing/assets/uc-spec.jpg";
import ucTriage from "../landing/assets/uc-triage.jpg";
import { Reveal } from "../landing/motion";
import { CONTAINER } from "../landing/layout";
import { H2 } from "../landing/primitives";
import { FEATURED, firstSentence } from "./catalog";
import { EntryLogo } from "./entry-card";

const ART = [
  plateTerrain,
  ucTriage,
  platePulses,
  plateBranch,
  plateFinal,
  plateGates,
  plateOrder,
  ucSpec,
];

export function Featured({ onOpen }: { onOpen: (id: string) => void }) {
  return (
    <section className={cn(CONTAINER, "pt-8 pb-24")}>
      <Reveal>
        <H2 size="sm">Featured</H2>
      </Reveal>
      <div className="mt-10 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {FEATURED.map((entry, index) => (
          <Reveal key={entry.id} delay={index * 0.05}>
            <button
              type="button"
              onClick={() => onOpen(entry.id)}
              className="group/feat block w-full cursor-pointer overflow-hidden rounded-[20px] border border-[color:var(--light-080)] bg-[color:var(--light-025)] p-0 text-left outline-none transition-[border-color] duration-300 hover:border-[color:var(--light-200)] focus-visible:ring-2 focus-visible:ring-brand"
            >
              <span className="relative grid h-[210px] place-items-center overflow-hidden bg-black">
                <img
                  src={ART[index % ART.length]}
                  alt=""
                  aria-hidden
                  draggable={false}
                  className="absolute inset-0 size-full object-cover opacity-75 transition-transform duration-[1200ms] ease-out group-hover/feat:scale-[1.07]"
                />
                <span
                  aria-hidden
                  className="absolute inset-x-0 bottom-0 h-20 bg-gradient-to-t from-[#0e0e0e] to-transparent"
                />
                <EntryLogo
                  entry={entry}
                  size="xl"
                  className="relative shadow-[inset_0_0_0_1px_rgba(255,255,255,0.14),0_30px_60px_-14px_rgba(0,0,0,0.9)]"
                />
              </span>
              <span className="block px-6 pt-4 pb-6">
                <span className="block text-[20px]/[1.25] font-medium text-fg">{entry.name}</span>
                <span className="mt-1.5 line-clamp-2 min-h-[44px] text-[14.5px]/[1.5] text-muted-foreground">
                  {firstSentence(entry.description)}
                </span>
              </span>
            </button>
          </Reveal>
        ))}
      </div>
    </section>
  );
}
