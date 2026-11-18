// 09 · 02 Tokens: the motion values rendered live (durations, eases, stagger, atmosphere and Live tokens).
import { cn } from "@compozy/ui/utils";

import { btn, btnSecondary, btnSm, card, h3, lab } from "../cls";
import { Section } from "../parts/section";
import { atmosphereTokens, bez, durationUse, easeUse, motion, type EaseName } from "./data";
import { ReducedNote, ReplayButton } from "./sig-card";
import { useLoopAnimation, useMotionDemo } from "./use-motion-demo";
import { SpecList } from "../parts/spec-card";

const durs = Object.entries(motion.durations);
const max = Math.max(...durs.map(([, v]) => v));
const eases = Object.entries(motion.eases) as [EaseName, number[]][];
// cubic-bezier as an SVG path in a 100×100 box (y up)
const curve = ([x1 = 0, y1 = 0, x2 = 1, y2 = 1]: number[]) =>
  `M0 100 C${x1 * 100} ${100 - y1 * 100} ${x2 * 100} ${100 - y2 * 100} 100 0`;
const rows = [
  "review-pr · claude-code",
  "migrate-db · codex",
  "triage-issues · cursor",
  "release-notes · gemini",
  "docs-sync · claude-code",
];
const json = JSON.stringify(motion, null, 2).replace(
  /\[\s+([^\]]*?)\s+\]/g,
  (_, c: string) => `[${c.replace(/\s+/g, " ")}]`
);

function Durations() {
  const { stageRef, play, reducedShown } = useMotionDemo("durations");
  return (
    <div className={card}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className={lab}>durations · ms</p>
        <button type="button" onClick={play} className={cn(btn, btnSm, btnSecondary)}>
          Play all
        </button>
      </div>
      <ReducedNote shown={reducedShown} className="mt-3" />
      <div ref={stageRef} data-demo="durations" className="mt-5 grid gap-4">
        {durs.map(([k, v]) => (
          <div
            key={k}
            className="grid items-center gap-x-5 gap-y-1 md:grid-cols-[150px_minmax(0,1fr)_minmax(0,1.1fr)]"
          >
            <p className="m-0 font-mono text-[13px]">
              <span className="text-(--b-fg)">{k}</span>{" "}
              <span className="text-(--b-accent)">{v}</span>
            </p>
            <div className="relative h-[10px] rounded-full bg-(--b-card-2)">
              <span
                className="absolute inset-y-0 left-0 block rounded-full bg-[linear-gradient(45deg,var(--color-brand-volt),var(--color-brand-sky))] opacity-60"
                style={{ width: `${Math.max(3, (v / max) * 100)}%` }}
              />
              <span
                data-dur={v}
                className="absolute top-0 block size-[10px] rounded-full bg-(--color-brand-cyan)"
                style={{ left: "calc(100% - 10px)" }}
              />
            </div>
            <p className="m-0 text-[14px] text-(--b-muted)">{durationUse[k]}</p>
          </div>
        ))}
      </div>
    </div>
  );
}

function EaseCard({ name, e }: { name: EaseName; e: number[] }) {
  // the dot rests 12% of the cycle at each end and travels on this ease in between
  const dot = useLoopAnimation<HTMLSpanElement>(
    [
      { left: "2px", offset: 0 },
      { left: "2px", offset: 0.12, easing: bez(e) },
      { left: "calc(100% - 12px)", offset: 0.88 },
      { left: "calc(100% - 12px)", offset: 1 },
    ],
    { duration: motion.durations.loop, iterations: Infinity, direction: "alternate" }
  );
  const [x1 = 0, y1 = 0, x2 = 1, y2 = 1] = e;
  const [use, text] = easeUse[name];
  return (
    <div className={card}>
      <p className={lab}>{use}</p>
      <h3 className={cn(h3, "mt-3")}>{name}</h3>
      <p className="m-0 font-mono text-[12px] text-(--b-accent)">{bez(e)}</p>
      <svg
        viewBox="-6 -6 112 112"
        className="mt-4 block aspect-square w-full max-w-[220px] overflow-visible"
        aria-hidden="true"
      >
        <rect
          x="0"
          y="0"
          width="100"
          height="100"
          fill="none"
          stroke="var(--b-line)"
          strokeWidth=".6"
        />
        <path d="M0 100 L100 0" stroke="var(--b-line)" strokeWidth=".6" strokeDasharray="2 2" />
        <line
          x1="0"
          y1="100"
          x2={x1 * 100}
          y2={100 - y1 * 100}
          stroke="var(--color-disabled)"
          strokeWidth=".6"
        />
        <line
          x1="100"
          y1="0"
          x2={x2 * 100}
          y2={100 - y2 * 100}
          stroke="var(--color-disabled)"
          strokeWidth=".6"
        />
        <path d={curve(e)} fill="none" stroke={`url(#mt-g-${name})`} strokeWidth="2.2" />
        <defs>
          <linearGradient id={`mt-g-${name}`} x1="0" y1="1" x2="1" y2="0">
            <stop offset="0" stopColor="var(--color-brand-volt)" />
            <stop offset="1" stopColor="var(--color-brand-cyan)" />
          </linearGradient>
        </defs>
      </svg>
      <div className="relative mt-5 h-[14px] rounded-full bg-(--b-card-2)">
        <span
          ref={dot}
          className="absolute top-[2px] left-[2px] block size-[10px] rounded-full bg-(--color-brand-cyan)"
        />
      </div>
      <p className="m-0 mt-4 text-[14px] text-(--b-muted)">{text}</p>
    </div>
  );
}

function Stagger() {
  const { stageRef, play, reducedShown } = useMotionDemo("stagger");
  return (
    <div className={card}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className={lab}>
          stagger · list {motion.stagger.list} ms · reveal {motion.stagger.reveal} ms
        </p>
        <ReplayButton onClick={play} />
      </div>
      <ReducedNote shown={reducedShown} className="mt-3" />
      <div
        ref={stageRef}
        data-demo="stagger"
        data-step={motion.stagger.list}
        className="mt-5 grid gap-2"
      >
        {rows.map(t => (
          <div
            key={t}
            data-st
            className="flex items-center justify-between rounded-[10px] border border-(--b-line) bg-(--b-card-2) px-3 py-2 font-mono text-[12.5px]"
          >
            <span>{t}</span>
            <span className="text-(--color-brand-cyan)">● running</span>
          </div>
        ))}
      </div>
      <p className="m-0 mt-4 text-[14px] text-(--b-muted)">
        Lists stagger 40 ms per row and stop after 8 rows: row 9 onwards enters with row 8. Headline
        words stagger 80 ms so each word reads as its own step.
      </p>
    </div>
  );
}

export function Tokens() {
  return (
    <Section
      id="tokens"
      n="02"
      file="brand/motion.json"
      title={
        <>
          Six durations, three <em>eases</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          All motion reads from one file, <code>brand/motion.json</code>. A value that is not in the
          file is not a brand value.
        </p>
      }
    >
      <Durations />
      <div className="mt-4 grid gap-4 md:grid-cols-3">
        {eases.map(([k, e]) => (
          <EaseCard key={k} name={k} e={e} />
        ))}
      </div>
      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <Stagger />
        <div className={card}>
          <p className={lab}>atmosphere and live · {atmosphereTokens.length} tokens</p>
          <SpecList rows={atmosphereTokens} />
          <p className="m-0 mt-4 text-[14px] text-(--b-muted)">
            Drift is measured on the visual’s own box, so it reads the same in a 16:9 cover and a
            9:16 story. Reduced motion turns drift and sweep off.
          </p>
        </div>
      </div>
      <pre className="mt-4 overflow-x-auto rounded-card border border-(--b-line) bg-(--b-card) p-6 font-mono text-[12.5px] leading-[1.7] text-(--b-muted)">
        {json}
      </pre>
    </Section>
  );
}
