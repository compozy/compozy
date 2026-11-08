// Formats, grid: the margin rule, 12/4 columns, the 8 px scale and a per-canvas table.
import { cn } from "@compozy/ui/utils";

import { cap, card, h3, lab } from "../cls";
import { Frame } from "../kit-frame";
import { Section } from "../parts/section";
import { canvases, columnsOf, gridRules, gutterOf, logoOf, marginOf, spacing } from "./data";
import { GridFrame } from "./grid-frame";

const by = (id: string) => {
  const c = canvases.find(x => x.id === id);
  if (!c) throw new Error(`unknown canvas ${id}`);
  return c;
};
const px = canvases.filter(c => !c.unit);

function SpacingScale() {
  return (
    <div className={card}>
      <p className={h3}>Spacing scale</p>
      <ul className="m-0 mt-4 grid list-none gap-2.5 p-0">
        {spacing.map(([s, use]) => (
          <li
            key={s}
            className="grid grid-cols-[44px_minmax(0,1fr)_minmax(0,11rem)] items-center gap-3"
          >
            <span className="font-mono text-[13px]">{s}</span>
            <span
              className="h-3 rounded-sm bg-[linear-gradient(45deg,var(--color-brand-volt),var(--color-brand-sky))]"
              style={{ width: `${(s / 128) * 100}%` }}
            />
            <span className="text-[13px] text-(--b-muted)">{use}</span>
          </li>
        ))}
      </ul>
      <p className={cap}>
        Values on a 1080–1600 px canvas. On 4K double them; on 800 px drop one step.
      </p>
    </div>
  );
}

function GridTable() {
  return (
    <div className="min-w-0 overflow-x-auto rounded-card border border-(--b-line)">
      <table className="w-full min-w-[520px] border-collapse text-left text-[14px]">
        <thead>
          <tr className="border-b border-(--b-line) bg-(--b-card)">
            {["Canvas", "Margin", "Columns", "Gutter", "Logo h"].map(t => (
              <th key={t} scope="col" className={cn("px-4 py-2.5 font-medium", lab)}>
                {t}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {px.map(c => (
            <tr key={c.id} className="border-b border-(--b-line) last:border-0">
              <th scope="row" className="px-4 py-2 font-medium">
                {c.name}{" "}
                <span className="font-mono text-[11.5px] font-normal text-(--b-muted)">
                  {c.w}×{c.h}
                </span>
              </th>
              <td className="px-4 py-2 font-mono">{marginOf(c)}</td>
              <td className="px-4 py-2 font-mono">{columnsOf(c)}</td>
              <td className="px-4 py-2 font-mono">{gutterOf(c)}</td>
              <td className="px-4 py-2 font-mono">{logoOf(c)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function Grid() {
  return (
    <Section
      id="grid"
      n="02"
      file="grid.css"
      title={
        <>
          One margin, two grids, <em>8 px</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          One margin rule for every canvas and a spacing scale in steps of 8 px. Landscape canvases
          get 12 columns, everything else 4. Lay out on the grid first, then place the visual behind
          it.
        </p>
      }
    >
      <div className="grid items-end gap-4 lg:grid-cols-[minmax(0,1.9fr)_minmax(0,1fr)_minmax(0,0.62fr)]">
        <figure className="m-0 min-w-0">
          <GridFrame c={by("x-post")} />
          <figcaption className={cap}>
            16:9 · 12 columns. The headline spans 6, the visual owns the rest.
          </figcaption>
        </figure>
        <figure className="m-0 min-w-0">
          <GridFrame c={by("li-post")} visual="converge" />
          <figcaption className={cap}>4:5 · 4 columns. Text spans 3, low on the canvas.</figcaption>
        </figure>
        <figure className="m-0 min-w-0">
          <GridFrame c={by("story")} visual="swarm" x={0.8} y={0.36} bands={[13, 19.8]} />
          <figcaption className={cap}>9:16 · 4 columns, plus the story bands.</figcaption>
        </figure>
      </div>

      <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
        <figure className="m-0 min-w-0">
          <Frame id="x-post" guides caption={false} />
          <figcaption className={cap}>
            The real kit template with its guides. Every template in the kit follows the same
            margin.
          </figcaption>
        </figure>
        <div className={card}>
          <p className={h3}>Four rules</p>
          <dl className="m-0 grid gap-3">
            {gridRules.map(([k, v]) => (
              <div key={k}>
                <dt className="font-medium">{k}</dt>
                <dd className="m-0 text-[14.5px] text-(--b-muted)">{v}</dd>
              </div>
            ))}
          </dl>
        </div>
      </div>

      <div className="mt-4 grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)]">
        <SpacingScale />
        <GridTable />
      </div>
    </Section>
  );
}
