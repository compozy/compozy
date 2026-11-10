import { cn } from "@compozy/ui";
import { CalendarClock, Eye, FileText, Hand, Webhook } from "@compozy/ui/icons";
import { m, useMotionValueEvent, useTransform, type MotionValue } from "motion/react";
import { useState } from "react";

import agents from "./assets/s-agents.jpg";
import heartbeat from "./assets/s-heartbeat.jpg";
import proposal from "./assets/s-proposal.jpg";
import reports from "./assets/s-reports.jpg";
import boardCrop from "./assets/board-crop.jpg";
import board from "./assets/board.jpg";
import order from "./assets/plate-order.jpg";
import tangle from "./assets/plate-tangle.jpg";
import watch from "./assets/plate-watch.jpg";
import { AgentAvatar } from "../always-on-agents-shared/agent-avatar";
import { Reveal, ScrollWords } from "./motion";
import { usePinning } from "./hooks/use-pinning";
import { useStage } from "./hooks/use-stage";
import { useProblemStage } from "./hooks/use-problem-stage";
import { CONTAINER, HAIRLINE, STAGE_FADE, type Focus } from "./layout";
import {
  Camera,
  LandingEyebrow,
  H2,
  Lead,
  Muted,
  Plate,
  PrimaryCta,
  ProductWindow,
  Shot,
  Small,
} from "./primitives";

const PANES = [
  {
    id: "waiting",
    name: "claude",
    lines: ["Waiting for your input…", "was this the payments repo?"],
    x: -27,
    y: -21,
    r: -5,
  },
  { id: "approval", name: "codex", lines: ["Allow command? [y/N]", "▌"], x: 26, y: -24, r: 4 },
  {
    id: "editing",
    name: "cursor",
    lines: ["Editing checkout/retry.ts", "Applying 14 changes…"],
    x: -31,
    y: 21,
    r: 3,
  },
  {
    id: "expired",
    name: "claude",
    lines: ["Session expired.", "$ tmux attach -t 4"],
    x: 0,
    y: 27,
    r: -2,
  },
  {
    id: "limited",
    name: "codex",
    lines: ["Rate limited. Retrying in 42s", "$ crontab -e"],
    x: 30,
    y: 19,
    r: -5,
  },
] as const;

function Terminal({ pane }: { pane: (typeof PANES)[number] }) {
  return (
    <div className="overflow-hidden rounded-[16px] border border-[color:var(--light-200)] bg-[#0d0d0d] shadow-[0_40px_90px_-10px_#000,0_0_0_1px_rgba(0,0,0,0.6)]">
      <div className="flex items-center gap-1.5 border-b border-[color:var(--light-100)] px-3.5 py-2.5">
        {[0, 1, 2].map(dot => (
          <span key={dot} className="size-2.5 rounded-full bg-[color:var(--light-200)]" />
        ))}
        <span className="ml-2 font-mono text-[12.5px] text-muted-foreground">{pane.name}</span>
      </div>
      <div className="p-5 font-mono text-[14.5px] leading-[1.7] text-fg/70">
        <div className="text-fg">$ {pane.name}</div>
        {pane.lines.map(line => (
          <div key={line}>{line}</div>
        ))}
      </div>
    </div>
  );
}

function FlyingPane({
  pane,
  index,
  progress,
}: {
  pane: (typeof PANES)[number];
  index: number;
  progress: MotionValue<number>;
}) {
  const start = 0.08 + index * 0.03;
  const x = useTransform(progress, [start, 0.54], [`${pane.x}vw`, "0vw"]);
  const y = useTransform(progress, [start, 0.54], [`${pane.y}vh`, "0vh"]);
  const rotate = useTransform(progress, [start, 0.54], [pane.r, 0]);
  const scale = useTransform(progress, [start, 0.54], [1, 0.42]);
  const opacity = useTransform(progress, [0.46, 0.56], [1, 0]);
  return (
    <m.div
      style={{ x, y, rotate, scale, opacity }}
      className="absolute top-1/2 left-1/2 -mt-20 -ml-[clamp(170px,12.5vw,240px)] w-[clamp(340px,25vw,480px)]"
    >
      <Terminal pane={pane} />
    </m.div>
  );
}

function ProblemStage() {
  const {
    ref,
    progress,
    tangleOpacity,
    tangleScale,
    orderOpacity,
    boardOpacity,
    boardScale,
    beforeOpacity,
    afterOpacity,
    lineOpacity,
    lineY,
  } = useProblemStage();
  return (
    <div ref={ref} className="relative h-[340vh]">
      <div className="sticky top-0 flex h-screen items-center justify-center overflow-hidden bg-[#0a0a0a]">
        <Plate
          src={tangle}
          style={{ opacity: tangleOpacity, scale: tangleScale }}
          className={cn("inset-0 size-full", STAGE_FADE)}
        />
        <Plate
          src={order}
          style={{ opacity: orderOpacity }}
          className={cn("inset-0 size-full", STAGE_FADE)}
        />
        <m.p
          style={{ opacity: beforeOpacity }}
          className="absolute top-[72px] left-1/2 z-10 m-0 -translate-x-1/2 rounded-pill border border-[color:var(--light-150)] bg-[color:var(--dark-700)] px-4 py-1.5 font-mono text-[13px] text-fg backdrop-blur-md"
        >
          Before
        </m.p>
        <m.p
          style={{ opacity: afterOpacity }}
          className="absolute top-[72px] left-1/2 z-10 m-0 -translate-x-1/2 rounded-pill border border-[color:var(--light-150)] bg-[color:var(--dark-700)] px-4 py-1.5 font-mono text-[13px] text-brand-strong backdrop-blur-md"
        >
          After
        </m.p>
        {PANES.map((pane, index) => (
          <FlyingPane key={pane.id} pane={pane} index={index} progress={progress} />
        ))}
        <m.div
          style={{ opacity: boardOpacity, scale: boardScale }}
          className="relative w-[min(1500px,84vw,calc(76vh*1.6))]"
        >
          <ProductWindow>
            <img
              src={board}
              alt="One sessions board sorted by who owns the next step"
              draggable={false}
              className="block size-full object-cover"
            />
          </ProductWindow>
        </m.div>
        <m.p
          style={{ opacity: lineOpacity, y: lineY }}
          className="absolute inset-x-0 bottom-9 m-0 text-center font-display text-[34px] tracking-[-0.02em] text-fg [text-shadow:0_2px_30px_#000] md:text-[44px]"
        >
          CompozyOS replaces the pile, not one tool in it.
        </m.p>
      </div>
    </div>
  );
}

export function Problem() {
  const pinning = usePinning();
  return (
    <section id="problem" className={cn("relative border-t", HAIRLINE)}>
      <div className={cn(CONTAINER, "pt-24 md:pt-44")}>
        <Reveal>
          <LandingEyebrow>The problem</LandingEyebrow>
        </Reveal>
        <H2 size="lg" className="max-w-[1100px]">
          <ScrollWords text="The bottleneck stopped being the model. It became managing the sessions." />
        </H2>
        <Reveal>
          <Lead className="max-w-[780px]">
            Run Claude Code, Codex, and Cursor in parallel and you end up with five terminals, a
            prompt typed into the wrong one, and no clear view of which agent is waiting on you.
            Keeping them working means gluing loops, cron, webhooks, and approvals together by hand.
          </Lead>
        </Reveal>
      </div>
      {pinning ? (
        <ProblemStage />
      ) : (
        <div className={cn(CONTAINER, "mt-14 pb-24")}>
          <div className="grid gap-3 sm:grid-cols-2">
            {PANES.map(pane => (
              <Terminal key={pane.id} pane={pane} />
            ))}
          </div>
          <Shot src={boardCrop} alt="One sessions board" className="mt-5" />
          <p className="m-0 mt-10 font-display text-[30px]/[1.15] tracking-[-0.02em] text-fg">
            CompozyOS replaces the pile, not one tool in it.
          </p>
        </div>
      )}
    </section>
  );
}

const BEATS: ReadonlyArray<{
  label: string;
  icon: typeof Eye;
  body: string;
  src: string;
  alt: string;
  focus?: Focus;
}> = [
  {
    label: "Check-ins",
    icon: CalendarClock,
    body: "On a schedule, the agent reviews its project in a hidden, read-only session. A quiet check costs no model call.",
    src: heartbeat,
    alt: "An agent's heartbeat page: next check, today's quiet checks, and why each one stayed silent",
  },
  {
    label: "Sources",
    icon: Webhook,
    body: "Point it at what it should watch: new GitHub issues from an extension, or a script you approve.",
    src: heartbeat,
    alt: "An agent's Sources list watching new GitHub issues in a repository",
    focus: { x: 0.14, y: 0.31, w: 0.46 },
  },
  {
    label: "Reports",
    icon: FileText,
    body: "Reports land in its main session. Come back after a day away and one card sums up what happened.",
    src: reports,
    alt: "A While you were away card: ten quiet hours, two decisions, with a check-in timeline",
  },
  {
    label: "Proposals",
    icon: Hand,
    body: "Pick a proposed action, or let the agent do it without asking, up to a daily limit you set.",
    src: proposal,
    alt: "Two proposals from the agent, each waiting for someone to pick it",
  },
];

const FACES = [
  "ci-doctor",
  "infra-drift",
  "pr-shepherd",
  "risk-monitor",
  "release-watch",
  "log-triage",
  "dependency-watch",
  "issue-triager",
  "orchestrator",
  "runbook-keeper",
  "security-scan",
  "ux-regression",
  "checkout-engineer",
  "docs-steward",
];

function Faces() {
  return (
    <div className="flex flex-wrap gap-2.5">
      {FACES.map((seed, index) => (
        <m.span
          key={seed}
          initial={{ opacity: 0, y: 14, scale: 0.7 }}
          whileInView={{ opacity: 1, y: 0, scale: 1 }}
          viewport={{ once: true }}
          transition={{ delay: index * 0.04, type: "spring", stiffness: 260, damping: 18 }}
        >
          <AgentAvatar
            appearance={{ avatar: "generated", seed }}
            label={seed}
            size={52}
            state={index % 5 === 4 ? "needs-you" : "working"}
            surfaceClassName="ring-[#0a0a0a]"
          />
        </m.span>
      ))}
    </div>
  );
}

function AlwaysOnStage() {
  const { ref, progress } = useStage();
  const [active, setActive] = useState(0);
  useMotionValueEvent(progress, "change", value =>
    setActive(Math.min(BEATS.length - 1, Math.floor(value * BEATS.length)))
  );
  const bar = useTransform(progress, value => (value * BEATS.length) % 1);
  return (
    <div ref={ref} className="relative h-[420vh]">
      <div className="sticky top-0 flex h-screen items-center overflow-hidden bg-[#070707]">
        <Plate src={watch} className={cn("inset-0 size-full opacity-70", STAGE_FADE)} />
        <div className="relative mx-auto grid w-full max-w-[1720px] grid-cols-[340px_minmax(0,1fr)] items-center gap-12 px-10">
          <ol className="m-0 list-none space-y-1 p-0">
            {BEATS.map((beat, index) => {
              const on = index === active;
              return (
                <li
                  key={beat.label}
                  className={cn(
                    "relative overflow-hidden rounded-[16px] border px-5 py-4 transition-colors duration-500",
                    on
                      ? "border-[color:var(--light-150)] bg-[color:var(--light-060)] backdrop-blur-xl"
                      : "border-transparent"
                  )}
                >
                  <p
                    className={cn(
                      "m-0 flex items-center gap-2.5 text-[18px] transition-colors",
                      on ? "text-fg" : "text-fg/40"
                    )}
                  >
                    <beat.icon
                      className={cn("size-[18px]", on ? "text-brand-strong" : "text-fg/40")}
                      aria-hidden
                    />
                    {beat.label}
                  </p>
                  <div
                    className={cn(
                      "grid transition-[grid-template-rows,opacity] duration-500",
                      on ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0"
                    )}
                  >
                    <div className="overflow-hidden">
                      <Muted className="pt-2 text-fg/75">{beat.body}</Muted>
                    </div>
                  </div>
                  {on ? (
                    <m.span
                      aria-hidden
                      style={{ scaleX: bar }}
                      className="absolute inset-x-0 bottom-0 h-[2px] origin-left bg-[linear-gradient(90deg,#1f87fc,#1df0d7)]"
                    />
                  ) : null}
                </li>
              );
            })}
          </ol>
          <div className="ml-auto w-full max-w-[calc(86vh*1.6)]">
            <ProductWindow>
              {BEATS.map((beat, index) => (
                <Camera
                  key={beat.label}
                  src={beat.src}
                  alt={beat.alt}
                  active={index === active}
                  focus={beat.focus}
                />
              ))}
            </ProductWindow>
          </div>
        </div>
      </div>
    </div>
  );
}

export function AlwaysOn() {
  const pinning = usePinning();
  return (
    <section id="always-on" className={cn("relative border-t bg-[#070707]", HAIRLINE)}>
      <div
        className={cn(CONTAINER, "grid gap-10 pt-24 md:pt-44 lg:grid-cols-[1.15fr_1fr] lg:gap-20")}
      >
        <div>
          <Reveal>
            <LandingEyebrow>Always-on</LandingEyebrow>
          </Reveal>
          <H2 size="lg">
            <ScrollWords text="Agents that check in on their own." />
          </H2>
        </div>
        <Reveal className="lg:pt-14">
          <Lead className="mt-0">
            An always-on agent has a name, a face, and one main session per project. It reviews its
            work on a schedule, stays quiet when there is nothing to say, and calls you when
            something needs a decision.
          </Lead>
          <div className="mt-8">
            <Faces />
          </div>
        </Reveal>
      </div>
      <div className={cn(CONTAINER, "mt-14 md:mt-20")}>
        <Reveal>
          <Shot
            src={agents}
            alt="The Agents view: always-on and on-demand agents, each with a face and what it is doing now"
          />
        </Reveal>
      </div>
      {pinning ? (
        <AlwaysOnStage />
      ) : (
        <div className={cn(CONTAINER, "mt-14 space-y-10")}>
          {BEATS.map(beat => (
            <div key={beat.label}>
              <p className="m-0 flex items-center gap-2.5 text-[18px] text-fg">
                <beat.icon className="size-[18px] text-brand-strong" aria-hidden />
                {beat.label}
              </p>
              <Muted className="mt-2 mb-5">{beat.body}</Muted>
              <ProductWindow>
                <Camera src={beat.src} alt={beat.alt} focus={beat.focus} />
              </ProductWindow>
            </div>
          ))}
        </div>
      )}
      <div className={cn(CONTAINER, "pt-16 pb-24 md:pb-40")}>
        <div
          className={cn(
            "flex flex-wrap items-center justify-between gap-8 border-t pt-10",
            HAIRLINE
          )}
        >
          <div className="max-w-[560px] space-y-2">
            <Muted>Agents keep notes on purpose. Nothing is extracted behind your back.</Muted>
            <Muted>
              Always-on next to On-demand: keep a few agents watching, start the rest when you need
              them.
            </Muted>
          </div>
          <div className="flex flex-col items-start gap-3 md:items-end">
            <PrimaryCta>Create an always-on agent</PrimaryCta>
            <Small>Check-ins need Claude, Codex, or OpenCode.</Small>
          </div>
        </div>
      </div>
    </section>
  );
}
