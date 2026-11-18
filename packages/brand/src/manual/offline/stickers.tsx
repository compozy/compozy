// 11 · 03 Stickers: the eight shapes, sizes and finishes, on the A5 sheet template.
import { cn } from "@compozy/ui/utils";

import { card, h3, rules } from "../cls";
import { Frame } from "../kit-frame";
import { Section } from "../parts/section";
import { stickers } from "./data";
import { TableHead } from "../parts/table-head";

export function Stickers() {
  return (
    <Section
      id="stickers"
      n="03"
      file="templates/offline/StickerSheet.astro"
      title={
        <>
          Shapes, faces, one <em>sheet</em>
        </>
      }
      bg="paper"
      status="proposal"
      intro={
        <p>
          Stickers travel further than any ad: laptops, water bottles, conference tables. Each shape
          is already a brand element from the product or the visual system, printed on one A5
          kiss-cut sheet. The faces of the roster agents, the always-on crew, are the sticker people
          ask for: small, friendly and recognisable from the app.
        </p>
      }
    >
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)]">
        <Frame id="sticker-sheet" guides />
        <div className="grid content-start gap-4">
          <div className="overflow-hidden rounded-card border border-(--b-line) bg-(--b-card)">
            <table className="w-full border-collapse text-[14px]">
              <TableHead columns={["Sticker", "Size", "Finish"]} />
              <tbody>
                {stickers.map(s => (
                  <tr key={s.name} className="border-b border-(--b-line) last:border-0">
                    <td className="px-5 py-2.5 font-medium">{s.name}</td>
                    <td className="px-5 py-2.5 font-mono text-[12.5px]">{s.size}</td>
                    <td className="px-5 py-2.5 text-(--b-muted)">{s.finish}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className={card}>
            <h3 className={h3}>Rules</h3>
            <ul className={cn(rules, "mt-3 text-[15px]")}>
              <li>
                Matte by default, so stickers photograph without glare. Gloss only on request, for
                the Field crop if you want its lit points to catch light.
              </li>
              <li>
                2 mm border inside every cut line: thin borders peel at the corners within a week.
              </li>
              <li>
                The gradient never fills a pill. Pills are ink or paper; the gradient lives in the
                symbol, the Field crop and one dot.
              </li>
              <li>
                Agent faces are for agents only, never people. Use the roster faces as drawn in the
                product (checkout-lead, qa-runner, release-manager …); a new face is a new agent,
                not a decoration.
              </li>
              <li>
                Text stickers stay under five words and use only lines from the messaging chapter.
              </li>
              <li>
                Minimum symbol sticker 25 mm; below that the symbol’s fine detail closes up in
                print.
              </li>
            </ul>
          </div>
        </div>
      </div>
    </Section>
  );
}
