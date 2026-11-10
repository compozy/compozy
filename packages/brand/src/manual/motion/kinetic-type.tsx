// 09 · 04 Kinetic type: word-by-word headlines, line masks and mono scramble, live.
import { cn } from "@compozy/ui/utils";

import { card, h3, lab, rules } from "../cls";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import { kineticRules } from "./data";
import { ReducedNote, ReplayButton } from "./sig-card";
import { useMotionDemo } from "./use-motion-demo";

const words = ["Keep", "every", "agent", "running."];
const labels = ["RELEASE 0.6 · BETA", "3 AGENTS · 1 NEEDS YOU", "SESSIONS / AGENTS / BOARD"];
const readingTime: [line: string, words: number][] = [
  ["Install the beta.", 3],
  ["Five terminals, and the prompt in the wrong window.", 9],
  ["Keep every agent running.", 4],
  ["The autonomous control plane for your agents.", 7],
];

function DemoHead({ label, onReplay }: { label: string; onReplay: () => void }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <p className={lab}>{label}</p>
      <ReplayButton onClick={onReplay} />
    </div>
  );
}

function Words() {
  const { stageRef, play, reducedShown } = useMotionDemo("words");
  return (
    <div className={card}>
      <DemoHead label="headline · word by word · 260 ms · stagger 80 ms" onReplay={play} />
      <div
        ref={stageRef}
        data-demo="words"
        className="mt-6 grid min-h-[260px] content-end rounded-[14px] bg-rail p-[6%]"
      >
        <p className="m-0 mono-label tracking-[0.12em] text-(--color-brand-cyan)">
          Always-on agents
        </p>
        <p className="m-0 mt-3 max-w-[13em] font-display text-[clamp(32px,4.2vw,58px)] leading-[1.02] tracking-[-0.03em]">
          {words.map(w => (
            <span key={w}>
              <span data-w className="inline-block">
                {w.startsWith("running") ? (
                  <>
                    <em data-em>running</em>.
                  </>
                ) : (
                  w
                )}
              </span>{" "}
            </span>
          ))}
        </p>
      </div>
      <ReducedNote shown={reducedShown} className="mt-3" />
    </div>
  );
}

function Scramble() {
  const { stageRef, play, reducedShown } = useMotionDemo("scramble");
  return (
    <div className={card}>
      <DemoHead label="mono scramble · 40 ms / char" onReplay={play} />
      <ReducedNote shown={reducedShown} className="mt-3" />
      <div
        ref={stageRef}
        data-demo="scramble"
        className="mt-5 grid gap-2.5 rounded-[14px] bg-rail p-5 mono-label text-[13px] tracking-[0.12em]"
      >
        {labels.map((l, i) => (
          <p
            key={l}
            data-scr={l}
            className={cn(
              "m-0 min-h-[1.5em] whitespace-nowrap",
              i === 0 ? "text-(--color-brand-cyan)" : "text-muted-foreground"
            )}
          >
            {l}
          </p>
        ))}
      </div>
    </div>
  );
}

function Lines() {
  const { stageRef, play, reducedShown } = useMotionDemo("lines");
  return (
    <div className={card}>
      <DemoHead label="line mask · 640 ms · stagger 160 ms" onReplay={play} />
      <ReducedNote shown={reducedShown} className="mt-3" />
      <div
        ref={stageRef}
        data-demo="lines"
        className="mt-5 rounded-[14px] bg-rail p-5 font-display text-[26px] leading-[1.1] tracking-[-0.02em]"
      >
        <p data-line className="m-0">
          Working, needs you,
        </p>
        <p data-line className="m-0">
          in review, done.
        </p>
        <p data-line className="m-0 text-muted-foreground">
          Then the next task.
        </p>
      </div>
    </div>
  );
}

export function KineticType() {
  return (
    <Section
      id="kinetic-type"
      n="04"
      file="motion/kinetic-type.md"
      title={
        <>
          Words arrive, then they <em>hold</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          Type moves only to set reading order. Display lines arrive word by word, labels resolve
          one character at a time, and then everything holds long enough to read. The words do the
          work; the motion only paces them.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <Words />
        <div className="grid gap-4">
          <Scramble />
          <Lines />
        </div>
      </div>

      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <div className={card}>
          <h3 className={h3}>Rules</h3>
          <ul className={cn(rules, "mt-3 text-[15px]")}>
            {kineticRules.map(r => (
              <li key={r}>{r}</li>
            ))}
          </ul>
        </div>
        <div className={card}>
          <h3 className={h3}>Reading time</h3>
          <table className="mt-3 w-full border-collapse text-[14px]">
            <thead>
              <tr className="border-b border-(--b-line) text-left mono-label text-[11.5px] tracking-[0.08em] text-(--b-muted)">
                <th className="py-2 font-medium">Line</th>
                <th className="py-2 font-medium">Words</th>
                <th className="py-2 text-right font-medium">Hold</th>
              </tr>
            </thead>
            <tbody>
              {readingTime.map(([l, w]) => (
                <tr key={l} className="border-b border-(--b-line)">
                  <td className="py-2 pr-3">{l}</td>
                  <td className="py-2 font-mono">{w}</td>
                  <td className="py-2 text-right font-mono text-(--b-accent)">
                    {Math.max(1.2, 0.6 + 0.3 * w).toFixed(1)} s
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <p className="m-0 mt-3 text-[14px] text-(--b-muted)">
            Hold = 0.6 s + 0.3 s per word, minimum 1.2 s.{" "}
          </p>
        </div>
      </div>

      <DoDont
        className="mt-4"
        doContent={
          <>
            <p className="m-0 font-display text-[22px] leading-[1.15]">
              Keep every agent <em>running</em>.
            </p>
            <p className="m-0 mt-2 text-[14px] text-(--b-muted)">
              Words rise 0.3 em and settle. The gradient word lights last.
            </p>
          </>
        }
        dontContent={
          <>
            <p className="m-0 font-display text-[22px] leading-[1.15] tracking-[0.3em]">
              A G E N T S
            </p>
            <p className="m-0 mt-2 text-[14px] text-(--b-muted)">
              Letter-by-letter, tracking animation, typewriter on display type, rotating words.
            </p>
          </>
        }
      />
    </Section>
  );
}
