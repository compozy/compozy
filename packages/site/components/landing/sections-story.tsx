/** The problem (a pile of terminals) and the answer (agents that check in on their own). */
import { cn } from "@compozy/ui";

import { ASSET } from "@/components/landing/assets";
import { HREF } from "@/components/landing/content";
import { Pinned } from "@/components/landing/pinned";
import {
  BEATS,
  FALLBACK_WINDOW,
  PANES,
  PILE_LINE,
  beatSizes,
} from "@/components/landing/story-data";
import { AlwaysOnStage, Faces, ProblemStage } from "@/components/landing/story-stages";
import { TerminalPane } from "@/components/landing/terminal-pane";
import { CONTAINER, HAIRLINE } from "@/components/site/layout";
import { Reveal, ScrollWords } from "@/components/site/motion";
import {
  Camera,
  H2,
  LandingEyebrow,
  Lead,
  Muted,
  PrimaryCta,
  ProductWindow,
  Shot,
  Small,
} from "@/components/site/primitives";

function ProblemFallback() {
  return (
    <div className={cn(CONTAINER, "mt-14 pb-24")}>
      <div className="grid gap-3 sm:grid-cols-2">
        {PANES.map(pane => (
          <TerminalPane key={pane.id} pane={pane} />
        ))}
      </div>
      <Shot src={ASSET.boardCrop} alt="One sessions board" className="mt-5" />
      <p className="m-0 mt-10 font-display text-[30px]/[1.15] tracking-[-0.02em] text-fg">
        {PILE_LINE}
      </p>
    </div>
  );
}

export function Problem() {
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
      <Pinned stage={<ProblemStage />} fallback={<ProblemFallback />} />
    </section>
  );
}

function AlwaysOnFallback() {
  return (
    <div className={cn(CONTAINER, "mt-14 space-y-10")}>
      {BEATS.map(beat => (
        <div key={beat.label}>
          <p className="m-0 flex items-center gap-2.5 text-[18px] text-fg">
            <beat.icon className="size-[18px] text-brand-strong" aria-hidden />
            {beat.label}
          </p>
          <Muted className="mt-2 mb-5">{beat.body}</Muted>
          <ProductWindow>
            <Camera
              src={beat.src}
              alt={beat.alt}
              focus={beat.focus}
              sizes={beatSizes(FALLBACK_WINDOW, beat.focus)}
            />
          </ProductWindow>
        </div>
      ))}
    </div>
  );
}

export function AlwaysOn() {
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
            src={ASSET.agents}
            alt="The Agents view: always-on and on-demand agents, each with a face and what it is doing now"
          />
        </Reveal>
      </div>
      <Pinned stage={<AlwaysOnStage />} fallback={<AlwaysOnFallback />} />
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
            <PrimaryCta href={HREF.alwaysOn}>Create an always-on agent</PrimaryCta>
            <Small>Check-ins need Claude, Codex, or OpenCode.</Small>
          </div>
        </div>
      </div>
    </section>
  );
}
