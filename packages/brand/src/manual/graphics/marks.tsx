// 05 Marks: section numbers, mono file labels, crop and registration marks, slide counters.
import { cn } from "@compozy/ui/utils";

import { VISUALS_BASE } from "../art/visuals";
import { cap, card, h3, lab } from "../cls";
import { Section } from "../parts/section";
import { marks } from "./data";
import { RuleCard } from "../parts/rule-card";

// crop marks on a 360×240 sheet with 18 px bleed (≈3 mm at this scale)
const W = 360;
const H = 240;
const O = 44;
const B = 18;
const L = 22;
const SHEET_W = W + O * 2;
const SHEET_H = H + O * 2;
const corners = [
  [O, O, -1, -1],
  [O + W, O, 1, -1],
  [O, O + H, -1, 1],
  [O + W, O + H, 1, 1],
] as const;
const targets: [number, number][] = [
  [SHEET_W / 2, 14],
  [SHEET_W / 2, SHEET_H - 14],
  [14, SHEET_H / 2],
  [SHEET_W - 14, SHEET_H / 2],
];
const slides = [1, 4, 12];

function RegMark({ x, y }: { x: number; y: number }) {
  return (
    <g transform={`translate(${x} ${y})`} className="stroke-rail" strokeWidth=".6" fill="none">
      <circle r="7" />
      <circle r="3.5" />
      <path d="M-11 0H11M0 -11V11" />
    </g>
  );
}

function PrintSheet() {
  return (
    <svg
      viewBox={`0 0 ${SHEET_W} ${SHEET_H}`}
      className="block h-auto w-full"
      role="img"
      aria-label="A print sheet with bleed, crop marks in each corner and registration targets on the sides"
    >
      <rect x={O - B} y={O - B} width={W + B * 2} height={H + B * 2} className="fill-rail" />
      <image
        href={`${VISUALS_BASE}/terrain-1280.jpg`}
        x={O - B}
        y={O - B}
        width={W + B * 2}
        height={H + B * 2}
        preserveAspectRatio="xMidYMid slice"
      />
      <rect
        x={O}
        y={O}
        width={W}
        height={H}
        fill="none"
        className="stroke-brand-cyan"
        strokeWidth=".6"
        strokeDasharray="3 3"
      />
      {corners.map(([x, y, sx, sy]) => (
        <path
          key={`${x}:${y}`}
          d={`M${x + sx * (B + 4)} ${y}h${sx * L}M${x} ${y + sy * (B + 4)}v${sy * L}`}
          className="stroke-rail"
          strokeWidth=".6"
        />
      ))}
      {targets.map(([x, y]) => (
        <RegMark key={`${x}:${y}`} x={x} y={y} />
      ))}
      <text x={O} y={SHEET_H - 6} className="fill-disabled font-mono" fontSize="7">
        postcard-a6 · CMYK · 3 mm
      </text>
    </svg>
  );
}

function PageMarks() {
  return (
    <div className="grid gap-4 lg:grid-cols-3">
      <div className={card}>
        <p className={lab}>Section number</p>
        <div className="mt-6 grid gap-4">
          {marks.sections.map(([n, f, t]) => (
            <div
              key={n}
              className="flex items-center gap-2.5 border-b border-(--b-line) pb-3 mono-label font-medium text-(--b-muted)"
            >
              <b className="font-medium text-(--b-accent)">{n}</b>
              <span>{f}</span>
              <span className="ml-auto tracking-normal normal-case">{t}</span>
            </div>
          ))}
        </div>
        <p className={cap}>
          Two digits, accent colour, then the file label. Geist Mono 500, 12 px, +0.1 em, uppercase.
        </p>
      </div>
      <div className={card}>
        <p className={lab}>File label</p>
        <ul className="m-0 mt-6 grid list-none gap-2 p-0">
          {marks.files.map(f => (
            <li
              key={f}
              className="w-fit rounded-md border border-(--b-line) px-2 py-0.5 font-mono text-[12.5px] text-(--b-fg)"
            >
              {f}
            </li>
          ))}
        </ul>
        <p className={cap}>
          Real paths, lowercase, never translated. The label names the artefact the section is
          about.
        </p>
      </div>
      <div className={card}>
        <p className={lab}>Slide counter</p>
        <div className="mt-6 grid gap-5">
          {slides.map(s => (
            <div key={s}>
              <div className="flex justify-between mono-label tracking-[0.08em] normal-case text-(--b-muted)">
                <span>
                  <b className="font-medium text-(--b-fg)">{String(s).padStart(2, "0")}</b> / 12
                </span>
                <span>release.md</span>
              </div>
              <div className="mt-2 h-px bg-(--b-line)">
                <div
                  className="h-px bg-[linear-gradient(45deg,var(--color-brand-volt),var(--color-brand-sky))]"
                  style={{ width: `${(s / 12) * 100}%` }}
                />
              </div>
            </div>
          ))}
        </div>
        <p className={cap}>
          Zero-padded current, total in muted, a 1 px progress hairline. Bottom-left on slides,
          top-right in carousels.
        </p>
      </div>
    </div>
  );
}

export function Marks() {
  return (
    <Section
      id="marks"
      n="05"
      file="marks.txt"
      title={
        <>
          Marks that say <em>where</em> you are
        </>
      }
      bg="paper"
      status="proposal"
      intro={
        <p>
          Small typographic marks organise a page the way a file tree organises a repo: number, file
          name, position. They are mono, small and quiet, and they never carry the message.
        </p>
      }
    >
      <PageMarks />

      <h3 className={cn(h3, "mt-16")}>Crop and registration, for print</h3>
      <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
        <figure className="m-0 min-w-0">
          <div className="rounded-card border border-(--b-line) bg-fg-strong p-4">
            <PrintSheet />
          </div>
          <figcaption className={cap}>
            A6 postcard sheet: art runs into the bleed, the dashed cyan line is the trim (shown here
            only), marks sit outside the bleed.
          </figcaption>
        </figure>
        <RuleCard title="Print marks">
          <li>
            <strong>Bleed</strong> 3 mm on every side; art and colour fields run to it, type stays 5
            mm inside trim.
          </li>
          <li>
            <strong>Crop marks</strong> 0.25 pt registration black, 5 mm long, offset 3 mm from the
            trim so they never print on the piece.
          </li>
          <li>
            <strong>Registration targets</strong> centred on each side, outside the bleed, so the
            printer can check plate alignment.
          </li>
          <li>
            <strong>Slug line</strong> in Geist Mono 6 pt: file name, colour mode, bleed, date. It
            identifies the proof on the press.
          </li>
          <li>
            <strong>As decoration</strong>, crop marks may frame a cover once, in line colour, never
            in the gradient.
          </li>
        </RuleCard>
      </div>
    </Section>
  );
}
