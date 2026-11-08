import { cn } from "@compozy/ui/utils";

import { card, lab, rules } from "../cls";
import { Frame } from "../kit-frame";
import { Section } from "../parts/section";
import { carouselBodies, carouselMap } from "./data";

export function Carousel() {
  return (
    <Section
      id="carousel"
      n="04"
      file="carousel.md"
      title={
        <>
          Carousels: one <em>step</em> per slide
        </>
      }
      intro={
        <p>
          A carousel tells one story in six slides at 1080×1350: the pain, the control plane,
          always-on agents, handoff, approvals, then the action. LinkedIn shows it as a document; X
          shows the first four as an image set. Every body slide pairs one claim with one Live
          visual of the product.
        </p>
      }
    >
      <div className="grid grid-cols-2 gap-4 md:grid-cols-3 lg:grid-cols-6">
        <Frame id="carousel-cover" guides />
        {carouselBodies.map(b => (
          <Frame key={b.step} id="carousel-body" />
        ))}
        <Frame id="carousel-cta" />
      </div>
      <div className="mt-12 grid gap-4 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
        <div className={card}>
          <p className={lab}>Slide map</p>
          <div className="mt-4 grid gap-0">
            {carouselMap.map(([n, name, d, id]) => (
              <div
                key={n}
                className="grid grid-cols-[64px_100px_minmax(0,1fr)] gap-4 border-b border-(--b-line) py-3 text-[15px] last:border-0"
              >
                <span className="font-mono text-(--b-accent)">{n}</span>
                <span className="font-semibold">{name}</span>
                <span className="text-(--b-muted)">
                  {d} <span className="font-mono text-[12px] text-(--b-accent)">{id}</span>
                </span>
              </div>
            ))}
          </div>
        </div>
        <div className={card}>
          <p className={lab}>Rules</p>
          <ul className={cn(rules, "mt-4 text-[15px]")}>
            <li>
              Six slides. More than eight and completion drops; fewer than four and it should be a
              single post.
            </li>
            <li>
              Body text at least 3.1% of the width (33 px): readable on a phone without zooming.
            </li>
            <li>
              One Live visual per body slide, drawn from the product: desktop, agents, handoff,
              approval. No terminal window.
            </li>
            <li>
              Export as one PDF for LinkedIn, as PNGs for X. Title the PDF with the cover line.
            </li>
          </ul>
        </div>
      </div>
    </Section>
  );
}
