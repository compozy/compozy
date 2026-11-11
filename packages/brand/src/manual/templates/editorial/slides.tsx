// Talk slides, 1920×1080: title, section divider, content, code and diagram. Every slide
// shares the mono footer (deck title left, page number right).
import { StateGlyph, type StateGlyphState } from "@compozy/ui";
import { cn } from "@compozy/ui/utils";

import { Visual } from "../../art/visual";
import { Mark, Scrim, ShotPanel } from "../parts";
import { cq, display, ink, monoUp, scrim } from "../styles";
import { withOffsets } from "../../text-runs";

const DECK = "The autonomous control plane";

function SlideChrome({ deck, page }: { deck: string; page: string }) {
  return (
    <div
      className={`${monoUp} absolute flex items-center justify-between text-muted-foreground`}
      style={{
        left: cq(5),
        right: cq(5),
        bottom: cq(3.4),
        fontSize: ".85cqi",
        letterSpacing: ".12em",
      }}
    >
      <span data-field="deck">{deck}</span>
      <span data-field="page">{page}</span>
    </div>
  );
}

const leftFade = (mid: number, alpha: number, end: number) =>
  scrim("90deg", [
    [0, 1],
    [mid, alpha],
    [end, 0],
  ]);

/** Title slide: talk title, speaker and event, Atmosphere block on the right. */
export function SlideTitle({
  title = "The bottleneck stopped being the model",
  speaker = "Pedro Nauck · Compozy",
  event = "AI Engineering Meetup · São Paulo",
}: {
  title?: string;
  speaker?: string;
  event?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="block" x={0.85} y={0.5} />
      <Scrim style={leftFade(44, 0.86, 72)} />
      <div
        className="absolute flex flex-col"
        style={{ top: cq(5), bottom: cq(5), left: cq(5), width: cq(56) }}
      >
        <Mark size={2} className="self-start" />
        <p
          data-field="title"
          className={display}
          style={{ fontSize: "5.6cqi", lineHeight: 1, marginTop: "auto" }}
        >
          {title}
        </p>
        <p
          className={`${monoUp} text-muted-foreground`}
          style={{ fontSize: "1cqi", letterSpacing: ".12em", marginTop: "3cqi" }}
        >
          <span data-field="speaker" className="text-fg">
            {speaker}
          </span>{" "}
          · <span data-field="event">{event}</span>
        </p>
      </div>
    </div>
  );
}

/** Section divider: number and name on ink, Atmosphere horizon behind. */
export function SlideSection({
  n = "02",
  title = "Managing the sessions",
  deck = DECK,
  page = "08",
}: {
  n?: string;
  title?: string;
  deck?: string;
  page?: string;
}) {
  return (
    <div className={ink}>
      <Visual name="horizon" x={0.5} y={0.5} />
      <Scrim style={leftFade(40, 0.7, 75)} />
      <div className="absolute -translate-y-1/2" style={{ left: cq(5), top: "50%" }}>
        <p
          data-field="n"
          className="m-0 font-mono text-brand-cyan"
          style={{ fontSize: "1.4cqi", letterSpacing: ".12em" }}
        >
          {n}
        </p>
        <p
          data-field="title"
          className={display}
          style={{ fontSize: "6.4cqi", lineHeight: 1, marginTop: "1.4cqi", maxWidth: "9em" }}
        >
          {title}
        </p>
      </div>
      <SlideChrome deck={deck} page={page} />
    </div>
  );
}

/** Content slide: the claim as headline, up to three points, one product capture top right. */
export function SlideContent({
  eyebrow = "The pattern",
  title = "The bottleneck stopped being the model. It became the sessions.",
  p1 = "Five terminals, and the prompt lands in the wrong window.",
  p2 = "Agents that wait for you stall; agents that run alone drift.",
  p3 = "One control plane shows who has the next step.",
  deck = DECK,
  page = "09",
}: {
  eyebrow?: string;
  title?: string;
  p1?: string;
  p2?: string;
  p3?: string;
  deck?: string;
  page?: string;
}) {
  const points: [field: string, text: string][] = [
    ["p1", p1],
    ["p2", p2],
    ["p3", p3],
  ];
  return (
    <div className={ink}>
      <div className="absolute" style={{ top: cq(7), right: cq(-1) }}>
        <ShotPanel name="board" w={51} h={31} radius={1.2} fit="cover" x={0} y={0.3} />
      </div>
      <div className="absolute" style={{ top: cq(7), left: cq(5), width: cq(40) }}>
        <p
          data-field="eyebrow"
          className={`${monoUp} text-brand-cyan`}
          style={{ fontSize: "1cqi", letterSpacing: ".12em" }}
        >
          {eyebrow}
        </p>
        <p
          data-field="title"
          className={`${display} tracking-[-0.025em]`}
          style={{ fontSize: "3.3cqi", lineHeight: 1.04, marginTop: "1.6cqi" }}
        >
          {title}
        </p>
      </div>
      <ol
        className="absolute m-0 grid list-none grid-cols-3 p-0"
        style={{ left: cq(5), right: cq(5), bottom: cq(9), gap: cq(2.4) }}
      >
        {points.map(([field, text], i) => (
          <li key={field} className="border-t border-line-strong" style={{ paddingTop: "1.6cqi" }}>
            <p className="m-0 font-mono text-brand-cyan" style={{ fontSize: "1cqi" }}>
              0{i + 1}
            </p>
            <p
              data-field={field}
              className="m-0"
              style={{ fontSize: "1.8cqi", lineHeight: 1.35, marginTop: "1cqi" }}
            >
              {text}
            </p>
          </li>
        ))}
      </ol>
      <SlideChrome deck={deck} page={page} />
    </div>
  );
}

// The loop file, one line per row: [text, tone] segments (k = key, s = string, n = number, c = comment).
type Tone = "k" | "s" | "n" | "c" | "";
const TONE: Record<Tone, string> = {
  k: "text-brand-cyan",
  s: "text-brand-glint",
  n: "text-brand-sky",
  c: "text-faint",
  "": "",
};
const LOOP_LINES: (readonly [text: string, tone: Tone])[][] = [
  [
    ["nodes", "k"],
    [":", ""],
  ],
  [
    ["  - ", ""],
    ["id", "k"],
    [": fix", ""],
  ],
  [
    ["    ", ""],
    ["kind", "k"],
    [": goal", ""],
  ],
  [
    ["    ", ""],
    ["params", "k"],
    [":", ""],
  ],
  [
    ["      ", ""],
    ["agent", "k"],
    [": claude-code", ""],
  ],
  [
    ["      ", ""],
    ["objective", "k"],
    [": ", ""],
    ['"All tests green on main"', "s"],
  ],
  [
    ["      ", ""],
    ["judge", "k"],
    [":", ""],
  ],
  [
    ["        - ", ""],
    ["kind", "k"],
    [": command", ""],
  ],
  [
    ["          ", ""],
    ["check", "k"],
    [": ", ""],
    ['"bun test"', "s"],
  ],
  [
    ["      ", ""],
    ["max_turns", "k"],
    [": ", ""],
    ["5", "n"],
  ],
  [
    ["      ", ""],
    ["on_exhausted", "k"],
    [": escalate  ", ""],
    ["# ask a person", "c"],
  ],
];
const LOOP_ROWS = LOOP_LINES.map((segments, i) => ({ n: i + 1, runs: withOffsets(segments) }));

/** Code slide: one claim on the left, the file that proves it on the right (≤ 14 lines). */
export function SlideCode({
  title = "A loop is a file you can read.",
  note = "Objective, judge, budget and what happens when the budget runs out, in one definition you can diff.",
  file = "loops/fix-flaky-tests.yaml · excerpt",
  deck = DECK,
  page = "14",
}: {
  title?: string;
  note?: string;
  file?: string;
  deck?: string;
  page?: string;
}) {
  return (
    <div className={ink}>
      <div
        className="absolute flex flex-col"
        style={{ top: cq(7), bottom: cq(9), left: cq(5), width: cq(30) }}
      >
        <p
          data-field="title"
          className={`${display} tracking-[-0.025em]`}
          style={{ fontSize: "3.6cqi", lineHeight: 1.04 }}
        >
          {title}
        </p>
        <p
          data-field="note"
          className="m-0 text-muted-foreground"
          style={{ fontSize: "1.4cqi", lineHeight: 1.45, marginTop: "2cqi" }}
        >
          {note}
        </p>
      </div>
      <div
        className="absolute overflow-hidden border border-line-strong bg-canvas font-mono"
        style={{ top: cq(7), bottom: cq(9), left: cq(39), right: cq(5), borderRadius: "1cqi" }}
      >
        <div
          className="flex items-center justify-between border-b border-line text-muted-foreground"
          style={{ padding: "1cqi 1.6cqi", fontSize: ".9cqi" }}
        >
          <span data-field="file">{file}</span>
          <span>yaml</span>
        </div>
        <pre
          className="m-0 whitespace-pre"
          style={{ padding: "1.6cqi", fontSize: "1.25cqi", lineHeight: 1.75 }}
        >
          {LOOP_ROWS.map(row => (
            <span key={row.n} className="block">
              <span className="inline-block text-faint" style={{ width: "2.6em" }}>
                {row.n}
              </span>
              {row.runs.map(({ at, run: [text, tone] }) => (
                <span key={at} className={TONE[tone]}>
                  {text}
                </span>
              ))}
            </span>
          ))}
        </pre>
      </div>
      <SlideChrome deck={deck} page={page} />
    </div>
  );
}

const DIAGRAM_STATES: [state: StateGlyphState, label: string, tone: string][] = [
  ["running", "working", "text-brand-cyan"],
  ["attention", "needs you", "text-attn"],
  ["idle", "in review", "text-fg"],
  ["done", "done", "text-success"],
];

/** Diagram slide: one orchestrator session hands work to child sessions in other harnesses,
 *  they return pull requests; the legend uses the product's own status glyphs. */
export function SlideDiagram({
  title = "One session hands the work to many.",
  deck = DECK,
  page = "15",
}: {
  title?: string;
  deck?: string;
  page?: string;
}) {
  return (
    <div className={ink}>
      <p
        data-field="title"
        className={`${display} absolute tracking-[-0.025em]`}
        style={{ top: cq(7), left: cq(5), fontSize: "3.6cqi", lineHeight: 1.04, maxWidth: "18em" }}
      >
        {title}
      </p>
      <div className="absolute" style={{ left: cq(5), top: cq(16) }}>
        <ShotPanel name="orchestrate" w={90} h={28.25} radius={1.2} pad={1.2} />
      </div>
      <div
        className="absolute flex font-mono"
        style={{ left: cq(5), bottom: cq(6.2), gap: cq(1), fontSize: "1.05cqi" }}
      >
        {DIAGRAM_STATES.map(([state, label, tone]) => (
          <span
            key={label}
            className={cn("inline-flex items-center border border-line-strong", tone)}
            style={{ borderRadius: "99cqi", padding: ".45cqi 1.2cqi", gap: ".6cqi" }}
          >
            <StateGlyph state={state} still />
            {label}
          </span>
        ))}
      </div>
      <SlideChrome deck={deck} page={page} />
    </div>
  );
}
