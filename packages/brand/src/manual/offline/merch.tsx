// 11 · 04 Merch: tee, hoodie, cap, tote, mug, notebook. Position, size, ink and fabric for each.
import { cn } from "@compozy/ui/utils";

import { card, h3, lab } from "../cls";
import { Frame } from "../kit-frame";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import { merch } from "./data";

export function Merch() {
  return (
    <Section
      id="merch"
      n="04"
      file="offline/merch.md"
      title={
        <>
          Things people <em>keep</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          Merch earns its place by being worn for years. Black garments, small marks, one large
          visual crop on the back at most. Engineers wear what looks like it came from a good tool,
          not from a booth.
        </p>
      }
    >
      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
        <Frame id="tee-chest" guides />
        <div className={card}>
          <p className={lab}>tee · centre chest</p>
          <h3 className={cn(h3, "mt-3")}>250 × 120 mm, 70 mm below the collar</h3>
          <p className="m-0 text-[14.5px] text-(--b-muted)">
            The print file is the artwork alone on black; the template renders it at 300 dpi. The
            symbol prints as simulated process over a white underbase, the line “Keep every agent
            running” in white. Hoodies reuse the same file at 80%. Agent faces can go on the back or
            sleeve as a tonal one-colour print.
          </p>
        </div>
      </div>
      <div className="mt-6 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {merch.map(m => (
          <div key={m.item} className={card}>
            <h3 className={h3}>{m.item}</h3>
            <p className="m-0 text-[14px] text-(--b-muted)">{m.fabric}</p>
            <dl className="m-0 mt-4 grid border-t border-(--b-line) text-[13.5px]">
              {m.specs.map(([k, v]) => (
                <div
                  key={k}
                  className="grid grid-cols-[96px_minmax(0,1fr)] gap-3 border-b border-(--b-line) py-2"
                >
                  <dt className="font-mono text-[12px] text-(--b-muted)">{k}</dt>
                  <dd className="m-0">{v}</dd>
                </div>
              ))}
            </dl>
          </div>
        ))}
      </div>
      <DoDont
        className="mt-6"
        doContent={
          <div>
            <p className="m-0 font-display text-[19px] tracking-[-0.025em]">
              Small mark, good blank
            </p>
            <p className="m-0 mt-2 text-[14.5px] text-(--b-muted)">
              A 70 mm embroidered symbol on a heavyweight hoodie. Quality of the garment is the
              brand.
            </p>
          </div>
        }
        dontContent={
          <div>
            <p className="m-0 font-display text-[19px] tracking-[-0.025em]">
              Slogan across the chest
            </p>
            <p className="m-0 mt-2 text-[14.5px] text-(--b-muted)">
              Full-width headlines, URLs on sleeves, the gradient as an all-over dye, white tees
              with a blue logo.
            </p>
          </div>
        }
      />
    </Section>
  );
}
