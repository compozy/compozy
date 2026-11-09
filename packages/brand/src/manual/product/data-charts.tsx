// 05 Data: chart vocabulary. Ramp for magnitude, one accent for focus, hatch for unknown,
// dashes for projected. Every chart colour reads a @compozy/ui token or the ramp in data.ts.
import { cn } from "@compozy/ui/utils";
import { Fragment } from "react";

import { cap, card, lab } from "../cls";
import { Section } from "../parts/section";
import { daily, ramp, usage } from "./data";
import { TermRules } from "../parts/rule-card";

const C = {
  accent: "var(--color-brand-cyan)",
  quiet: "var(--color-disabled)",
  hatch: "var(--color-indicator)",
  series: "var(--color-brand-sky)",
  grid: "var(--color-line)",
  bar: "var(--color-line-strong)",
  label: "var(--color-muted-foreground)",
  fg: "var(--color-fg)",
  empty: "var(--color-elevated)",
  warn: "var(--color-warning)",
};
const mono = "var(--font-mono)";

// daily bars
const W = 640;
const H = 240;
const pad = { l: 36, r: 8, t: 16, b: 28 };
const max = 50;
const bw = (W - pad.l - pad.r) / daily.length;
const y = (v: number) => pad.t + (H - pad.t - pad.b) * (1 - v / max);
const ticks = [0, 25, 50];
// heatmap: 7 days × 12 weeks, deterministic values
const heat = Array.from({ length: 7 }, (_, d) =>
  Array.from({ length: 12 }, (_, w) =>
    Math.round(((Math.sin(w * 1.7 + d * 0.9) + 1) * 2.4 + w * 0.22 + (d < 5 ? 1 : -1)) % 7)
  ).map(v => Math.max(0, Math.min(6, v)))
);
const days = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
const totalCost = usage.reduce((a, u) => a + u.cost, 0);
const maxTok = Math.max(...usage.map(u => u.tokens));
const shares = [ramp[2], ramp[1], ramp[3], ramp[5]];
/** A Loop's attempts in order: ✕ failed, ✓ passed, · not run yet. */
const LOOP_ATTEMPTS = (["✕", "✕", "✓", "·", "·", "·", "·", "·"] as const).map((mark, i) => ({
  attempt: i + 1,
  mark,
}));
const meters: { name: string; v: number; of: string; warn: number; note: string }[] = [
  {
    name: "Context window · review-pr",
    v: 0.62,
    of: "124k / 200k tokens",
    warn: 0.8,
    note: "Turns amber at 80%: the agent may compact soon.",
  },
  {
    name: "Monthly budget",
    v: 0.84,
    of: "$74.10 / $88.00",
    warn: 0.8,
    note: "Amber past the threshold the operator set, red at 100%.",
  },
];
const signals: [title: string, body: string, key: string][] = [
  [
    "Gradient ramp",
    "Magnitude in one variable: heatmaps, density. Deep blue is low, glint is high.",
    "ramp",
  ],
  [
    "Single accent",
    "The series in question is cyan; every other series is #5e5e5e. One accent per chart.",
    "accent",
  ],
  [
    "Hatched",
    "Unknown or missing: the daemon was offline, the provider did not report. Never a zero.",
    "hatch",
  ],
  [
    "Dashed",
    "Projected or estimated. Outline only, the same hue as the series it extends.",
    "dash",
  ],
];

function SignalSwatch({ k }: { k: string }) {
  return (
    <svg viewBox="0 0 120 40" className="block h-10 w-full" aria-hidden="true">
      <defs>
        <pattern
          id={`h-${k}`}
          width="5"
          height="5"
          patternUnits="userSpaceOnUse"
          patternTransform="rotate(45)"
        >
          <line x1="0" y1="0" x2="0" y2="5" stroke={C.hatch} strokeWidth="1.5" />
        </pattern>
      </defs>
      {k === "ramp" &&
        ramp.map((c, i) => (
          <rect key={i} x={i * 17} y="6" width="16" height="28" rx="2" fill={c} />
        ))}
      {k === "accent" && (
        <>
          <polyline
            points="0,30 30,26 60,28 90,22 120,24"
            fill="none"
            stroke={C.quiet}
            strokeWidth="1.5"
          />
          <polyline
            points="0,32 30,24 60,18 90,12 120,6"
            fill="none"
            stroke={C.accent}
            strokeWidth="2"
          />
        </>
      )}
      {k === "hatch" && (
        <>
          <rect x="10" y="12" width="24" height="28" fill={C.series} />
          <rect x="48" y="4" width="24" height="36" fill={`url(#h-${k})`} stroke={C.hatch} />
          <rect x="86" y="16" width="24" height="24" fill={C.series} />
        </>
      )}
      {k === "dash" && (
        <>
          <rect x="10" y="16" width="24" height="24" fill={C.series} />
          <rect
            x="48"
            y="10"
            width="24"
            height="30"
            fill="none"
            stroke={C.series}
            strokeDasharray="3 2"
          />
          <rect
            x="86"
            y="4"
            width="24"
            height="36"
            fill="none"
            stroke={C.series}
            strokeDasharray="3 2"
          />
        </>
      )}
    </svg>
  );
}

function DailyBars() {
  return (
    <svg
      viewBox={`0 0 ${W} ${H}`}
      className="mt-4 block h-auto w-full"
      role="img"
      aria-label="Bar chart of sessions done per day: rising from 18 to 41, one day unknown, three days projected up to 49"
    >
      <defs>
        <pattern
          id="hatch-d"
          width="6"
          height="6"
          patternUnits="userSpaceOnUse"
          patternTransform="rotate(45)"
        >
          <line x1="0" y1="0" x2="0" y2="6" stroke={C.hatch} strokeWidth="1.5" />
        </pattern>
      </defs>
      {ticks.map(t => (
        <Fragment key={t}>
          <line x1={pad.l} x2={W - pad.r} y1={y(t)} y2={y(t)} stroke={C.grid} />
          <text
            x={pad.l - 8}
            y={y(t) + 4}
            textAnchor="end"
            fill={C.label}
            fontFamily={mono}
            fontSize="11"
          >
            {t}
          </text>
        </Fragment>
      ))}
      {daily.map((d, i) => {
        const x = pad.l + i * bw + bw * 0.18;
        const w = bw * 0.64;
        const last = i === 10;
        return (
          <Fragment key={d.d}>
            {d.v === null ? (
              <rect
                x={x}
                y={y(40)}
                width={w}
                height={y(0) - y(40)}
                fill="url(#hatch-d)"
                stroke={C.hatch}
                strokeWidth="1"
                rx="2"
              />
            ) : d.projected ? (
              <rect
                x={x + 0.5}
                y={y(d.v) + 0.5}
                width={w - 1}
                height={y(0) - y(d.v) - 1}
                fill="none"
                stroke={C.accent}
                strokeDasharray="4 3"
                rx="2"
              />
            ) : (
              <rect
                x={x}
                y={y(d.v)}
                width={w}
                height={y(0) - y(d.v)}
                fill={last ? C.accent : C.bar}
                rx="2"
              />
            )}
            <text
              x={x + w / 2}
              y={H - 10}
              textAnchor="middle"
              fill={last ? C.fg : C.label}
              fontFamily={mono}
              fontSize="10.5"
            >
              {d.d}
            </text>
            {last && d.v !== null && (
              <text
                x={x + w / 2}
                y={y(d.v) - 6}
                textAnchor="middle"
                fill={C.accent}
                fontFamily={mono}
                fontSize="11"
              >
                {d.v}
              </text>
            )}
            {d.v === null && (
              <text
                x={x + w / 2}
                y={y(40) - 6}
                textAnchor="middle"
                fill={C.label}
                fontFamily={mono}
                fontSize="10.5"
              >
                ?
              </text>
            )}
          </Fragment>
        );
      })}
    </svg>
  );
}

function Usage() {
  return (
    <div className={card}>
      <p className={lab}>Usage by agent · last 30 days · sample</p>
      <div className="mt-5 grid gap-3.5">
        {usage.map((u, i) => (
          <div
            key={u.agent}
            className="grid grid-cols-[96px_minmax(0,1fr)_64px] items-center gap-3 text-[13px]"
          >
            <span className="font-mono text-[12px]">{u.agent}</span>
            <span className="h-2.5 overflow-hidden rounded-full bg-(--color-rail)">
              <span
                className={cn(
                  "block h-full rounded-full",
                  i === 0 ? "[background:var(--gradient-live)]" : "bg-line-strong"
                )}
                style={{ width: `${(u.tokens / maxTok) * 100}%` }}
              />
            </span>
            <span className="text-right font-mono text-[12px] text-(--b-muted)">
              {u.tokens}M tok
            </span>
          </div>
        ))}
      </div>
      <div className="mt-6 border-t border-(--b-line) pt-4">
        <div className="flex justify-between font-mono text-[12px]">
          <span className="text-(--b-muted)">Cost</span>
          <span>${totalCost.toFixed(2)}</span>
        </div>
        <div className="mt-2 flex h-3 gap-0.5 overflow-hidden rounded-[4px]">
          {usage.map((u, i) => (
            <span
              key={u.agent}
              title={u.agent}
              style={{ width: `${(u.cost / totalCost) * 100}%`, background: shares[i] }}
            />
          ))}
        </div>
        <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 font-mono text-[11px] text-(--b-muted)">
          {usage.map((u, i) => (
            <span key={u.agent} className="flex items-center gap-1.5">
              <span className="size-2 rounded-[2px]" style={{ background: shares[i] }} />
              {u.agent} ${u.cost.toFixed(2)}
            </span>
          ))}
        </div>
      </div>
      <p className={cap}>
        The gradient bar is the one being asked about; cost shares use adjacent ramp steps with 2 px
        gaps and direct labels.
      </p>
    </div>
  );
}

function Meters() {
  return (
    <div className={card}>
      <p className={lab}>Meters</p>
      <div className="mt-5 grid gap-6">
        {meters.map(m => {
          const over = m.v >= m.warn;
          return (
            <div key={m.name}>
              <div className="flex justify-between text-[13.5px]">
                <span>{m.name}</span>
                <span
                  className={cn(
                    "font-mono text-[12px]",
                    over ? "text-warning" : "text-(--b-muted)"
                  )}
                >
                  {m.of}
                </span>
              </div>
              <div className="relative mt-2 h-2 rounded-full bg-(--color-rail)">
                <span
                  className="absolute inset-y-0 left-0 rounded-full"
                  style={{ width: `${m.v * 100}%`, background: over ? C.warn : C.accent }}
                />
                <span
                  className="absolute -top-1 -bottom-1 w-px bg-(--color-muted-foreground)"
                  style={{ left: `${m.warn * 100}%` }}
                />
              </div>
              <p className="m-0 mt-2 text-[13px] text-(--b-muted)">{m.note}</p>
            </div>
          );
        })}
        <div>
          <div className="flex justify-between text-[13.5px]">
            <span>Loop attempts · fix-flaky-auth</span>
            <span className="font-mono text-[12px] text-(--b-muted)">3 of 8</span>
          </div>
          <div className="mt-2 flex gap-1.5 font-mono text-[14px]">
            {LOOP_ATTEMPTS.map(({ attempt, mark: g }) => (
              <span
                key={attempt}
                className={cn(
                  "grid size-7 place-items-center rounded-[6px] border",
                  g === "✕"
                    ? "border-line-strong text-(--b-muted)"
                    : g === "✓"
                      ? "border-success text-success"
                      : "border-line text-(--color-disabled)"
                )}
              >
                {g}
              </span>
            ))}
          </div>
          <p className="m-0 mt-2 text-[13px] text-(--b-muted)">
            Discrete steps are cells, not a bar: a Loop runs attempts, not percent.
          </p>
        </div>
      </div>
    </div>
  );
}

export function DataCharts() {
  return (
    <Section
      id="data"
      n="05"
      file="usage.svg"
      title={
        <>
          Charts that <em>answer</em> one question
        </>
      }
      status="proposal"
      intro={
        <p>
          Usage, cost and throughput are where operators decide. Charts use four signals only: the
          gradient ramp for magnitude, one accent for the series that matters, hatching for what is
          unknown and dashes for what is projected.
        </p>
      }
    >
      <div className="grid gap-4 md:grid-cols-4">
        {signals.map(([t, d, k]) => (
          <div key={k} className={card}>
            <SignalSwatch k={k} />
            <p className="m-0 mt-4 text-[16px] font-medium">{t}</p>
            <p className="m-0 mt-1 text-[14px] text-(--b-muted)">{d}</p>
          </div>
        ))}
      </div>

      <div className="mt-6 grid gap-4 lg:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <figure className={cn(card, "m-0")}>
          <p className={lab}>Sessions done per day · sample</p>
          <DailyBars />
          <figcaption className={cap}>
            Today in the accent with its value; history in line-strong; Oct 28 hatched (daemon
            offline); the next three days dashed.
          </figcaption>
        </figure>
        <figure className={cn(card, "m-0")}>
          <p className={lab}>Activity · 12 weeks</p>
          <div className="mt-4 grid grid-cols-[32px_repeat(12,minmax(0,1fr))] gap-1">
            {heat.map((row, d) => (
              <Fragment key={d}>
                <span className="font-mono text-[10.5px] leading-[18px] text-(--b-muted)">
                  {days[d]}
                </span>
                {row.map((v, w) => (
                  <span
                    key={w}
                    className="aspect-square rounded-[3px]"
                    style={{ background: v === 0 ? C.empty : ramp[v] }}
                    title={`${v * 3} sessions`}
                  />
                ))}
              </Fragment>
            ))}
          </div>
          <div className="mt-4 flex items-center gap-1.5 font-mono text-[11px] text-(--b-muted)">
            less
            {[C.empty, ...ramp.slice(1)].map((c, i) => (
              <span key={i} className="size-3 rounded-[2px]" style={{ background: c }} />
            ))}
            more
          </div>
          <figcaption className={cap}>
            The ramp starts at ink-3 for zero, so empty days recede instead of reading as low.
          </figcaption>
        </figure>
      </div>

      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <Usage />
        <Meters />
      </div>
      <TermRules
        className="mt-10 max-w-[76ch] text-[15.5px]"
        items={[
          [
            "Label directly",
            ": values and series names sit on the chart. Legends only past four series.",
          ],
          [
            "Numbers in Geist Mono",
            ", tabular, with units: 124k tokens, $74.10, 06:31. Never a bare number.",
          ],
          ["Zero is a value", ", missing is hatched. A chart never draws an unknown as zero."],
          [
            "Status colours",
            " appear in charts only for thresholds the operator set: amber past a warning, red past a limit.",
          ],
        ]}
      />
    </Section>
  );
}
