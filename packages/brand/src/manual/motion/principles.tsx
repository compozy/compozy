// 09 · 01 Principles: what motion means here, and the reduced-motion version.
import { cn } from "@compozy/ui/utils";

import { Visual } from "../art/visual";
import { card, h3, lab } from "../cls";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import { motion, principles, reducedRules } from "./data";
import { SpecList } from "../parts/spec-card";

const pair = (title: string, text: string) => (
  <>
    <p className="m-0 font-display text-[19px]">{title}</p>
    <p className="m-0 mt-2 text-[14.5px] text-(--b-muted)">{text}</p>
  </>
);

export function Principles() {
  return (
    <Section
      id="principles"
      n="01"
      file="motion/principles.md"
      title={
        <>
          Motion shows work <em>running</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          Compozy keeps agents working after the tab closes. Motion is how that becomes visible: a
          bar grows to the now line, a status changes, light passes over a surface and settles into
          a state you can read. Nothing bounces, nothing performs.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
        <div className="relative min-h-[360px] overflow-hidden rounded-card border border-(--b-line)">
          <Visual name="swarm" x={0.7} y={0.4} className="visual-drift" />
          <div className="absolute inset-x-0 bottom-0 bg-[linear-gradient(0deg,color-mix(in_oklab,var(--color-rail)_95%,transparent),transparent)] p-6 pt-16">
            <p className={lab}>atmosphere/swarm · drift 24 s</p>
            <p className="m-0 mt-2 max-w-[38ch] text-[15px] text-(--b-muted)">
              The only motion that never stops: an Atmosphere image drifts 1.5% of its box and
              scales 1.02 → 1.05 over 24 s, then reverses. Everything else plays once and rests.
            </p>
          </div>
        </div>
        <ol className="m-0 grid list-none gap-3 p-0 sm:grid-cols-2">
          {principles.map(([n, t, d]) => (
            <li key={n} className={card}>
              <p className="m-0 font-mono text-[12px] text-(--b-accent)">{n}</p>
              <h3 className={cn(h3, "mt-3 text-[19px]")}>{t}</h3>
              <p className="m-0 text-[14.5px] text-(--b-muted)">{d}</p>
            </li>
          ))}
        </ol>
      </div>

      <div className="mt-10 grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
        <div className={card}>
          <p className={lab}>prefers-reduced-motion: reduce</p>
          <h3 className={cn(h3, "mt-3")}>The reduced version is designed, not removed</h3>
          <SpecList rows={reducedRules} />
        </div>
        <div className={card}>
          <p className={lab}>implementation</p>
          <pre className="m-0 mt-4 overflow-x-auto rounded-[12px] bg-rail p-4 font-mono text-[12.5px] leading-[1.7] text-muted-foreground">{`@media (prefers-reduced-motion: reduce) {
  * { animation: none !important;
      transition-duration: 0ms !important; }
}
const reduced = matchMedia(
  '(prefers-reduced-motion: reduce)').matches;
if (reduced) showFinalFrame();
else play(${motion.durations.sweep}, 'ease-out');`}</pre>
          <p className="m-0 mt-4 text-[14px] text-(--b-muted)">
            Every live demo on this page follows the same rule: markup starts in its final state,
            script rewinds and plays it. With reduced motion, the script does nothing.
          </p>
        </div>
      </div>

      <DoDont
        className="mt-10"
        doContent={pair(
          "Decelerate into rest",
          "Ease out, 260 ms, the card stops where it lands. The eye reads the state, not the trip."
        )}
        dontContent={pair(
          "Overshoot, spring, wobble",
          "A status pill that bounces past its spot and back looks like a toy and hides the state for 400 ms."
        )}
      />
      <DoDont
        className="mt-4"
        doContent={pair(
          "Light travels with the work",
          "Sweeps and growth move left to right, toward the now line. The direction of work is always readable."
        )}
        dontContent={pair(
          "Spinners and endless pulses",
          "A spinning loader says “wait” without saying what for. Show the step and the time instead: “Reading 14 files · 3m”."
        )}
      />
    </Section>
  );
}
