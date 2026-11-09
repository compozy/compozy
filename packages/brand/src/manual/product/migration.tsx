// 09 Migration: every surface the previous identity touched in the Compozy repo and its
// replacement. The retired orange is named in words only (UT-012).
import { cn } from "@compozy/ui/utils";
import { Fragment } from "react";

import { cap, card, h3, lab, rules } from "../cls";
import { Section } from "../parts/section";
import { migration, steps, swatchesAfter, swatchesBefore } from "./migration-data";

const hexes = (s: string) => s.match(/#[0-9A-Fa-f]{6}\b/g) ?? [];
const kept = /^Unchanged|\(kept\)/;

function Swatches({ text }: { text: string }) {
  return (
    <>
      {hexes(text).map((h, i) => (
        <span
          key={`${h}-${i}`}
          className="size-3 shrink-0 rounded-[3px] shadow-[inset_0_0_0_1px_var(--b-line)]"
          style={{ background: h }}
        />
      ))}
    </>
  );
}

function SwatchList({ items, retired }: { items: string[]; retired?: boolean }) {
  return (
    <div className="grid gap-1.5">
      {retired && (
        <span className="flex items-center gap-2 text-(--b-muted)">
          <span className="size-4 rounded-[4px] border border-dashed border-(--b-line)" />
          retired orange
        </span>
      )}
      {items.map(h => (
        <span key={h} className="flex items-center gap-2">
          <span
            className="size-4 rounded-[4px] shadow-[inset_0_0_0_1px_var(--b-line)]"
            style={{ background: h }}
          />
          {h}
        </span>
      ))}
    </div>
  );
}

function BeforeAfter() {
  return (
    <div className={card}>
      <p className={lab}>Before → after</p>
      <div className="mt-5 grid grid-cols-[1fr_auto_1fr] items-center gap-4">
        <div className="grid aspect-square place-items-center rounded-[22%] border border-dashed border-(--b-line) text-center">
          <span>
            <span className="block font-display text-[56px] leading-none text-(--b-muted)">ꝏ</span>
            <span className="mt-2 block mono-label text-[10px] text-(--b-muted)">retired</span>
          </span>
        </div>
        <span className="font-mono text-(--b-muted)">→</span>
        <img
          src="/brand/png/compozy-app-icon-256h.png"
          alt="New app icon"
          className="w-full rounded-[22%]"
        />
      </div>
      <div className="mt-5 grid grid-cols-[1fr_auto_1fr] items-center gap-4 font-mono text-[11.5px]">
        <SwatchList items={swatchesBefore} retired />
        <span className="text-(--b-muted)">→</span>
        <SwatchList items={swatchesAfter} />
      </div>
      <p className={cap}>
        The retired orange is gone from every surface: tokens, icons, site and video now use the
        blues and the ink ladder. Its job, the action colour, goes to volt on paper and signal blue
        on ink; its warmth is not replaced. Needs you keeps its own amber.
      </p>
    </div>
  );
}

export function Migration() {
  return (
    <Section
      id="migration"
      n="09"
      file="MIGRATION.md"
      title={
        <>
          From orange to <em>ink</em>, file by file
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          Every surface the current brand touches, with the path in the Compozy repo and its
          replacement in this package. Paths are relative to the repo root; replacements point at{" "}
          <code>brand/</code> here.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
        <BeforeAfter />
        <div className={card}>
          <p className={lab}>Order of work</p>
          <ol className="m-0 mt-4 grid list-none gap-0 p-0">
            {steps.map(([n, t, d]) => (
              <li
                key={n}
                className="grid grid-cols-[36px_110px_minmax(0,1fr)] gap-3 border-b border-(--b-line) py-3 text-[14.5px]"
              >
                <span className="font-mono text-[12px] text-(--b-accent)">{n}</span>
                <span className="font-medium">{t}</span>
                <span className="text-(--b-muted)">{d}</span>
              </li>
            ))}
          </ol>
        </div>
      </div>

      {migration.map(g => (
        <Fragment key={g.group}>
          <h3 className={cn(h3, "mt-14")}>{g.group}</h3>
          <div className="mt-4 overflow-x-auto rounded-card border border-(--b-line) bg-(--b-card)">
            <table className="w-full min-w-[980px] table-fixed border-collapse text-[13.5px]">
              <thead>
                <tr className="font-mono text-[11px] tracking-[0.08em] text-(--b-muted) uppercase">
                  <th className="w-[18%] px-5 py-3 text-left font-medium">Asset</th>
                  <th className="w-[28%] px-3 py-3 text-left font-medium">Current path</th>
                  <th className="w-[20%] px-3 py-3 text-left font-medium">Now</th>
                  <th className="px-3 py-3 text-left font-medium">Replace with</th>
                </tr>
              </thead>
              <tbody>
                {g.rows.map(([a, p, b, n]) => (
                  <tr key={a} className="border-t border-(--b-line) align-top">
                    <td className="px-5 py-2.5 font-medium">{a}</td>
                    <td className="px-3 py-2.5 font-mono text-[11.5px] break-words text-(--b-muted)">
                      {p}
                    </td>
                    <td className="px-3 py-2.5 text-[13px] text-(--b-muted)">
                      <span className="flex flex-wrap items-center gap-1.5">
                        <Swatches text={b} />
                        <span
                          className={cn(
                            !kept.test(n) && "line-through decoration-(--color-destructive)/50"
                          )}
                        >
                          {b}
                        </span>
                      </span>
                    </td>
                    <td className="px-3 py-2.5 text-[13px]">
                      <span className="flex flex-wrap items-center gap-1.5">
                        <Swatches text={n} />
                        <span>{n}</span>
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Fragment>
      ))}
      <ul className={cn(rules, "mt-10 max-w-[76ch] text-[15.5px]")}>
        <li>
          <strong>Names stay, values change.</strong> Components read <code>--color-brand</code>{" "}
          today; it keeps its name and points at volt, so the swap touches token files only.
          Renaming is a separate change.
        </li>
        <li>
          <strong>DESIGN.md is generated</strong> (make codegen): change the tokens, regenerate,
          never edit it by hand.
        </li>
        <li>
          <strong>The product name</strong> in UI stays as the glossary says; this table changes
          pixels, not words.
        </li>
      </ul>
    </Section>
  );
}
