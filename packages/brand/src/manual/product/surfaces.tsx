// 02 Surfaces: dark and light ladders mapped to tokens, with computed text contrast on every step.
import { cn } from "@compozy/ui/utils";

import { cap, h3, lab } from "../cls";
import { grade, ratio } from "../contrast";
import { UiCrop } from "../graphics/ui-crop";
import { Section } from "../parts/section";
import { darkLadder, darkText, ladderInk, lightLadder, lightText, type Surf } from "./data";
import { TermRules } from "../parts/rule-card";

interface Ladder {
  name: string;
  steps: Surf[];
  text: [string, string][];
  dark: boolean;
}
const ladders: Ladder[] = [
  { name: "Dark · default", steps: darkLadder, text: darkText, dark: true },
  { name: 'Light · data-theme="light"', steps: lightLadder, text: lightText, dark: false },
];

const hex = (pairs: [string, string][], i: number) => pairs[i]?.[1] ?? "";

function gradeTone(g: string, dark: boolean) {
  const ink = ladderInk.light;
  if (g === "fail") return dark ? { className: "text-destructive" } : { color: ink.fail };
  if (g === "AA large") return dark ? { className: "text-warning" } : { color: ink.large };
  return dark ? { className: "text-(--b-accent)" } : { color: ink.pass };
}

function Specimen({ L }: { L: Ladder }) {
  const ink = L.dark ? ladderInk.dark : ladderInk.light;
  const [rail, canvas, elevated, hover] = L.steps;
  const muted = hex(L.text, 1);
  return (
    <div
      className="mt-5 rounded-[14px] p-3"
      style={{ background: rail?.hex, boxShadow: `inset 0 0 0 1px ${ink.line}` }}
    >
      <p className="m-0 mb-2 font-mono text-[11px]" style={{ color: muted }}>
        rail
      </p>
      <div
        className="rounded-[12px] p-3"
        style={{ background: canvas?.hex, boxShadow: `inset 0 0 0 1px ${ink.line}` }}
      >
        <p className="m-0 mb-2 font-mono text-[11px]" style={{ color: muted }}>
          canvas
        </p>
        <div className="grid grid-cols-[1.4fr_1fr] gap-3">
          <div
            className="rounded-[10px] p-3"
            style={{ background: elevated?.hex, boxShadow: `inset 0 0 0 1px ${ink.lineStrong}` }}
          >
            <p className="m-0 font-mono text-[11px]" style={{ color: muted }}>
              elevated
            </p>
            <p
              className="m-0 mt-2 rounded-[6px] px-2 py-1 text-[13px]"
              style={{ background: hover?.hex, color: hex(L.text, 0) }}
            >
              hover · selected
            </p>
          </div>
          <div className="grid content-center gap-1 text-[13px]">
            {L.text.slice(0, 2).map(([n, h]) => (
              <span key={n} style={{ color: h }}>
                {n} text
              </span>
            ))}
            <span className="font-mono" style={{ color: hex(L.text, 3) }}>
              ● Working
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}

function LadderCard({ L }: { L: Ladder }) {
  return (
    <div
      className={cn(
        "min-w-0 rounded-card border border-(--b-line) bg-(--b-card) p-[22px] sm:p-7",
        !L.dark && "band-paper"
      )}
    >
      <p className={lab}>{L.name}</p>
      <Specimen L={L} />
      <div className="mt-5 overflow-x-auto">
        <table className="w-full min-w-[520px] border-collapse text-[13px]">
          <thead>
            <tr className="font-mono text-[11px] tracking-[0.08em] text-(--b-muted) uppercase">
              <th className="py-2 pr-2 text-left font-medium">Surface</th>
              {L.text.map(([n]) => (
                <th key={n} className="px-1 py-2 text-right font-medium">
                  {n}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {L.steps.map(s => (
              <tr key={s.token} className="border-t border-(--b-line)">
                <td className="py-2 pr-2">
                  <span className="flex items-center gap-2">
                    <span
                      className="inline-block size-5 rounded-[5px] shadow-[inset_0_0_0_1px_var(--b-line)]"
                      style={{ background: s.hex }}
                    />
                    <span>
                      <span className="block font-mono text-[11.5px]">{s.token}</span>
                      <span className="block font-mono text-[11px] text-(--b-muted)">{s.hex}</span>
                    </span>
                  </span>
                </td>
                {L.text.map(([n, h]) => {
                  const r = ratio(h, s.hex);
                  const g = grade(r);
                  const tone = gradeTone(g, L.dark);
                  return (
                    <td key={n} className="px-1 py-2 text-right font-mono text-[11.5px]">
                      <span className="mr-1 rounded px-1" style={{ color: h, background: s.hex }}>
                        Aa
                      </span>
                      {r.toFixed(1)}{" "}
                      <span
                        className={tone.className}
                        style={tone.color ? { color: tone.color } : undefined}
                      >
                        {g === "AA large" ? "AA-L" : g}
                      </span>
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <ul className="m-0 mt-4 grid list-none gap-1 p-0 text-[13px] text-(--b-muted)">
        {L.steps.map(s => (
          <li key={s.token}>
            <span className="font-mono">{s.token}</span> · {s.role}
            {s.old && <span> · replaces {s.old}</span>}
          </li>
        ))}
      </ul>
    </div>
  );
}

export function Surfaces() {
  return (
    <Section
      id="surfaces"
      n="02"
      file="surfaces.css"
      title={
        <>
          Four steps of <em>ink</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          The product stacks four surfaces in each theme. Each step is one notch lighter (dark) or
          darker (light) than the one beneath it, so depth reads without shadows. Every text colour
          is checked on every step.
        </p>
      }
    >
      <figure className="m-0 mb-10">
        <UiCrop name="desktop" ratio="16/9" />
        <figcaption className={cap}>
          The ladder in use: rail, canvas, elevated windows and hover, on the Compozy desktop. Depth
          comes from the four steps and hairlines, not from shadows.
        </figcaption>
      </figure>
      <div className="grid gap-6 xl:grid-cols-2">
        {ladders.map(L => (
          <LadderCard key={L.name} L={L} />
        ))}
      </div>
      <div className="mt-10 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <div>
          <h3 className={h3}>Reading the table</h3>
          <p className={cap}>
            Ratios are WCAG 2.2 relative luminance, computed at build from the hex values. AA-L
            means 3:1, enough for 18 px+ or bold 14 px+ and for icons, never for body copy.
          </p>
        </div>
        <TermRules
          className="text-[15px]"
          items={[
            [
              "Faint",
              " (#5e5e5e / #a6a6a6) is for disabled controls, placeholders’ icons and hairline labels only. It fails body contrast on purpose.",
            ],
            [
              "Volt as text",
              " uses #6687FF on dark: brand volt #2F5BFF is 3.7:1 on ink, fine for a filled button, too dark for a link.",
            ],
            [
              "No shadows",
              " to separate layers in dark. Popovers add 1 px line-strong and a 24 px, 40% black shadow only so they detach from the canvas.",
            ],
            [
              "Selected",
              " rows use ink-4 plus a 2 px cyan bar on the left edge, so selection never depends on fill alone.",
            ],
          ]}
        />
      </div>
    </Section>
  );
}
