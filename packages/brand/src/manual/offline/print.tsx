// 11 · 01 Print: colour on paper (CMYK and Pantone approximations), the gradient in print, stocks, white ink, files.
import { cn } from "@compozy/ui/utils";

import { card, h3, lab, rules } from "../cls";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import { fileSpecs, gradientPrint, inks, stocks, whiteInk } from "./data";
import { SpecList } from "../parts/spec-card";
import { TableHead } from "../parts/table-head";

/** How each print route reads on paper, painted from the brand tokens (screen previews, not proofs). */
const PRINT_PREVIEW = [
  "linear-gradient(135deg,color-mix(in oklab,var(--color-brand-deep) 80%,var(--color-muted-foreground)),color-mix(in oklab,var(--color-brand-sky) 75%,var(--color-muted-foreground)) 55%,color-mix(in oklab,var(--color-brand-cyan) 60%,var(--color-muted-foreground)))",
  "var(--gradient-brand)",
  "linear-gradient(135deg,var(--color-brand-volt),var(--color-success))",
  "var(--color-brand-volt)",
];

function InkTable() {
  return (
    <div className="mt-4 overflow-x-auto rounded-card border border-(--b-line) bg-(--b-card)">
      <table className="w-full min-w-[860px] border-collapse text-[14px]">
        <TableHead columns={["Colour", "Screen", "CMYK (coated)", "Pantone", "Note"]} />
        <tbody>
          {inks.map(c => (
            <tr key={c.name} className="border-b border-(--b-line) align-top last:border-0">
              <td className="px-5 py-3">
                <span
                  className="mr-2 inline-block size-3 rounded-full align-[-1px] shadow-[0_0_0_1px_color-mix(in_oklab,var(--color-muted-foreground)_40%,transparent)]"
                  style={{ background: c.swatch }}
                />
                {c.name}
              </td>
              <td className="px-5 py-3 font-mono text-[12.5px]">{c.hex}</td>
              <td className="px-5 py-3 font-mono text-[12.5px]">{c.cmyk}</td>
              <td className="px-5 py-3 font-mono text-[12.5px] text-(--b-accent)">{c.pantone}</td>
              <td className="px-5 py-3 text-(--b-muted)">{c.note}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function Print() {
  return (
    <Section
      id="print"
      n="01"
      file="offline/print.md"
      title={
        <>
          Light on paper is <em>ink</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          Screen colour is light; print is pigment. The gradient loses its glow on paper, so print
          pieces either accept a calmer gradient or buy the glow back with a fluorescent spot.
          Atmosphere visuals are dark rasters: print them full bleed on rich black, never on paper.
          Every value below is a starting point for a press proof.
        </p>
      }
    >
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
        {inks.map(c => (
          <div
            key={c.name}
            className="grid min-h-[150px] content-between rounded-[14px] border border-(--b-line) p-4"
            style={{ background: c.swatch, color: c.text }}
          >
            <span className="mono-label text-[11px]">{c.hex}</span>
            <span className="font-display text-[20px] tracking-[-0.025em]">{c.name}</span>
          </div>
        ))}
      </div>
      <InkTable />
      <p className="m-0 mt-2.5 text-sm text-(--b-muted)">
        Approximations, not certified matches. Approve every job on a printed proof under D50 light;
        the screen is never the reference for paper.
      </p>

      <h3 className={cn(h3, "mt-14")}>The gradient in print</h3>
      <div className="mt-4 grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {gradientPrint.map(([t, d], i) => (
          <div key={t} className={card}>
            <div
              className={cn("h-16 rounded-[10px]", i === 2 && "contrast-[1.05]")}
              style={{ background: PRINT_PREVIEW[i] }}
            />
            <p className={cn(lab, "mt-4")}>{i === 1 ? "recommended" : `option 0${i + 1}`}</p>
            <p className="m-0 mt-2 font-display text-[19px] tracking-[-0.025em]">{t}</p>
            <p className="m-0 mt-2 text-[14px] text-(--b-muted)">{d}</p>
          </div>
        ))}
      </div>

      <div className="mt-14 grid gap-4 lg:grid-cols-2">
        <div className={card}>
          <h3 className={h3}>Paper stocks</h3>
          <dl className="m-0 mt-3 grid text-[14px]">
            {stocks.map(([p, a, b]) => (
              <div
                key={p}
                className="grid grid-cols-[120px_minmax(0,1fr)] gap-3 border-b border-(--b-line) py-2.5"
              >
                <dt className="font-mono text-[12.5px] text-(--b-muted)">{p}</dt>
                <dd className="m-0">
                  {a}
                  <span className="block text-[13px] text-(--b-muted)">Alt: {b}</span>
                </dd>
              </div>
            ))}
          </dl>
        </div>
        <div className="grid gap-4">
          <div className={card}>
            <h3 className={h3}>White ink</h3>
            <ul className={cn(rules, "mt-3 text-[14.5px]")}>
              {whiteInk.map(r => (
                <li key={r}>{r}</li>
              ))}
            </ul>
          </div>
          <div className={card}>
            <h3 className={h3}>Files for the printer</h3>
            <SpecList rows={fileSpecs} className="mt-3" />
          </div>
        </div>
      </div>

      <DoDont
        className="mt-4"
        doContent={
          <div>
            <div className="h-20 rounded-[10px] bg-rail p-4 font-mono text-[12px] text-fg">
              C60 M50 Y50 K100 · rich black ground
            </div>
            <p className="m-0 mt-3 text-[14px] text-(--b-muted)">
              Large ink areas in rich black so they read as the screen’s neutral black.
            </p>
          </div>
        }
        dontContent={
          <div>
            <div className="h-20 rounded-[10px] bg-surface-2 p-4 font-mono text-[12px] text-fg-2">
              K 100 only · reads charcoal
            </div>
            <p className="m-0 mt-3 text-[14px] text-(--b-muted)">
              Single-ink black on a full card prints dull grey and shows every scratch.
            </p>
          </div>
        }
      />
    </Section>
  );
}
