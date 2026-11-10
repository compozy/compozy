// Formats, canvases: every canvas Compozy uses, drawn to one scale, then listed by channel.
import { cn } from "@compozy/ui/utils";

import { cap, card, h3, lab } from "../cls";
import { Section } from "../parts/section";
import { canvases, type Group } from "./data";

const strip = canvases.filter(c => c.strip && !c.unit);
const groups = [...new Set(canvases.map(c => c.group))] as Group[];
/** px of drawing per px of canvas in the strip */
const K = 0.13;

function RatioStrip() {
  return (
    <div className={card}>
      <p className={h3}>Ratios, side by side</p>
      <div
        className="mt-6 flex flex-wrap items-end gap-x-6 gap-y-8"
        role="img"
        aria-label="The main canvases drawn at the same scale: 16:9, 3:1, 4:5, 1:1, 9:16 and 1.91:1"
      >
        {strip.map(c => (
          <div key={c.id} className="grid gap-2">
            <div
              className="relative grid place-items-center rounded-[8px] border border-brand-cyan/60 bg-brand-cyan/6 font-mono text-[12px] text-brand-cyan"
              style={{
                width: `calc(${c.w}px * ${K} * var(--k,1))`,
                height: `calc(${c.h}px * ${K} * var(--k,1))`,
              }}
            >
              {c.ratio}
            </div>
            <span className="font-mono text-[11px] leading-[1.35] text-(--b-muted)">
              {c.name}
              <br />
              {c.w} × {c.h}
            </span>
          </div>
        ))}
      </div>
      <p className={cap}>
        Drawn at 13 % of real size. Same width does not mean same format: a 4:5 and a 1:1 at 1080
        wide are 270 px apart in height.
      </p>
    </div>
  );
}

function CanvasTable() {
  return (
    <div className="mt-4 overflow-x-auto rounded-card border border-(--b-line)">
      <table className="w-full min-w-[720px] border-collapse text-left text-[15px]">
        <thead>
          <tr className="border-b border-(--b-line) bg-(--b-card)">
            {["Canvas", "Shape", "Size", "Ratio", "Use"].map(t => (
              <th key={t} scope="col" className={cn("px-4 py-3 font-medium", lab)}>
                {t}
              </th>
            ))}
          </tr>
        </thead>
        {groups.map(g => (
          <tbody key={g}>
            <tr className="border-b border-(--b-line)">
              <th
                colSpan={5}
                scope="rowgroup"
                className="bg-(--b-card-2) px-4 py-2 text-left mono-label font-medium text-(--b-accent)"
              >
                {g}
              </th>
            </tr>
            {canvases
              .filter(c => c.group === g)
              .map(c => {
                const s = 30 / Math.max(c.w, c.h);
                return (
                  <tr key={c.id} className="border-b border-(--b-line) last:border-0">
                    <th scope="row" className="px-4 py-2.5 font-medium">
                      {c.name}
                    </th>
                    <td className="px-4 py-2.5">
                      <span
                        className="block rounded-[3px] border border-brand-cyan/70"
                        style={{ width: Math.max(6, c.w * s), height: Math.max(6, c.h * s) }}
                      />
                    </td>
                    <td className="px-4 py-2.5 font-mono text-[13.5px] whitespace-nowrap">
                      {c.w} × {c.h}
                      {c.unit ? " mm" : ""}
                    </td>
                    <td className="px-4 py-2.5 font-mono text-[13.5px] whitespace-nowrap text-(--b-accent)">
                      {c.ratio}
                    </td>
                    <td className="px-4 py-2.5 text-[14.5px] text-(--b-muted)">{c.use}</td>
                  </tr>
                );
              })}
          </tbody>
        ))}
      </table>
    </div>
  );
}

export function Canvas() {
  return (
    <Section
      id="canvas"
      n="01"
      file="canvases.md"
      title={
        <>
          Every canvas, at one <em>scale</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          Every Compozy piece starts on one of these {canvases.length} canvases. If none fits, the
          piece is the problem, not the list. Sizes are what the platform recommends today; export
          at exactly these pixels.
        </p>
      }
    >
      <RatioStrip />
      <CanvasTable />
      <div className="mt-4 grid gap-4 md:grid-cols-3">
        <div className={card}>
          <p className={lab}>Export</p>
          <p className="m-0 mt-3 text-[14.5px] text-(--b-muted)">
            PNG for anything with type or the symbol; JPG at 90 only for photographs. sRGB, never
            larger than the platform limit.
          </p>
        </div>
        <div className={card}>
          <p className={lab}>Retina</p>
          <p className="m-0 mt-3 text-[14.5px] text-(--b-muted)">
            Social canvases are already 2× their display size. Do not export at 2× again; the
            platform will recompress it.
          </p>
        </div>
        <div className={card}>
          <p className={lab}>Print</p>
          <p className="m-0 mt-3 text-[14.5px] text-(--b-muted)">
            Millimetres, 3 mm bleed, 300 dpi, CMYK proof of the gradient before ordering. Details in{" "}
            <a href="/offline" className="text-(--b-accent)">
              Offline →
            </a>
          </p>
        </div>
      </div>
    </Section>
  );
}
