/** Proof: the jobs it takes over, the people running it, how you supervise, how agents hand off. */
import { cn } from "@compozy/ui";
import {
  CalendarClock,
  CircleDot,
  GitBranch,
  GitPullRequest,
  Hand,
  ListChecks,
  Monitor,
  Package,
  Scale,
  Search,
  Star,
  Users,
  Workflow,
} from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { ASSET } from "@/components/landing/assets";
import { QUOTES } from "@/components/landing/content";
import { DetailPanels } from "@/components/landing/detail-panels";
import { LiveCount, LiveStars } from "@/components/landing/live-count";
import { CONTAINER, HAIRLINE } from "@/components/site/layout";
import { Reveal, ScrollWords } from "@/components/landing/motion";
import {
  H2,
  H3,
  LandingEyebrow,
  Lead,
  Muted,
  Plate,
  ProductWindow,
  Small,
  type Icon,
} from "@/components/site/primitives";

const USE_CASES: ReadonlyArray<{
  lane: string;
  icon: Icon;
  title: string;
  body: string;
  art: string;
  span: string;
}> = [
  {
    lane: "Dev",
    icon: CircleDot,
    title: "Triage new issues as they land",
    body: "An always-on agent watches new GitHub issues. It proposes what to do; you pick.",
    art: ASSET.ucTriage,
    span: "lg:col-span-7",
  },
  {
    lane: "Dev",
    icon: GitBranch,
    title: "Deliver a spec while you are away",
    body: "A conductor splits the work into slices. Each slice runs in its own worktree.",
    art: ASSET.ucSpec,
    span: "lg:col-span-5",
  },
  {
    lane: "Dev",
    icon: Search,
    title: "Review a change until the round is clean",
    body: "A reviewer finds, an implementer fixes. Findings land in a file you can open.",
    art: ASSET.ucReview,
    span: "lg:col-span-5",
  },
  {
    lane: "Ops",
    icon: CalendarClock,
    title: "Leave a briefing waiting every weekday",
    body: "A schedule starts an agent with nobody watching. Every run is recorded, with retry on failure.",
    art: ASSET.ucBrief,
    span: "lg:col-span-7",
  },
  {
    lane: "Ops",
    icon: ListChecks,
    title: "Keep release checks as claimed work",
    body: "Each concern is a task an agent claims. Runs close with evidence.",
    art: ASSET.ucClaims,
    span: "lg:col-span-7",
  },
  {
    lane: "Ops",
    icon: Hand,
    title: "Wait for a person before a sensitive action",
    body: "A tool call can require approval before it runs. Waiting work rises to Needs you.",
    art: ASSET.ucGate,
    span: "lg:col-span-5",
  },
];

export function UseCases() {
  return (
    <section id="use-cases" className="relative py-24 md:py-40">
      <div className={CONTAINER}>
        <div className="mx-auto max-w-[920px] text-center">
          <Reveal>
            <LandingEyebrow>Use cases</LandingEyebrow>
          </Reveal>
          <H2>
            <ScrollWords text="CompozyOS takes the jobs you keep doing by hand." />
          </H2>
        </div>
        <div className="mt-16 grid gap-4 md:mt-24 lg:grid-cols-12">
          {USE_CASES.map((item, index) => (
            <Reveal key={item.title} delay={index * 0.05} className={item.span}>
              <article className="group relative h-[420px] overflow-hidden rounded-[22px] border border-site-light-090 bg-black">
                <img
                  src={item.art}
                  alt=""
                  aria-hidden
                  draggable={false}
                  className="absolute inset-0 size-full object-cover opacity-85 transition-transform duration-[1400ms] ease-out group-hover:scale-110"
                />
                <div
                  aria-hidden
                  className="absolute inset-0 bg-[linear-gradient(180deg,rgba(10,10,10,0)_30%,rgba(10,10,10,0.92)_78%)]"
                />
                <div className="relative flex h-full flex-col justify-between p-7 md:p-8">
                  <p className="m-0 inline-flex w-fit items-center gap-2 rounded-pill border border-site-light-150 bg-site-dark-450 py-1.5 pr-3.5 pl-2.5 font-mono text-[12.5px] text-fg backdrop-blur-md">
                    <item.icon className="size-3.5 text-brand-strong" aria-hidden />
                    {item.lane}
                  </p>
                  <div className="max-w-[520px]">
                    <H3>{item.title}</H3>
                    <Muted className="mt-3 text-fg/75">{item.body}</Muted>
                  </div>
                </div>
              </article>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}

const QUOTE_STYLE = [
  "lg:col-span-5 bg-[linear-gradient(45deg,#1f87fc,#1df0d7_70%,#b5faf6)] text-[#0a0a0a]",
  "lg:col-span-7 bg-[#2448e6] text-white",
  "lg:col-span-12 border border-site-light-100 bg-[#111] text-fg",
];

const STATS: ReadonlyArray<{ icon: Icon; value: ReactNode; label: string }> = [
  { icon: Star, value: <LiveStars />, label: "on GitHub" },
  { icon: Scale, value: "MIT", label: "license" },
  { icon: Package, value: null, label: "releases" },
  { icon: Monitor, value: "Local-first", label: "" },
];

/** `releases` is the count of published releases, `null` when GitHub is unavailable. */
export function Community({ releases }: { releases: number | null }) {
  return (
    <section id="community" className={cn("relative border-t py-24 md:py-40", HAIRLINE)}>
      <div className={CONTAINER}>
        <div className="text-center">
          <Reveal>
            <LandingEyebrow icon={Users}>Community</LandingEyebrow>
          </Reveal>
          <H2>
            <ScrollWords text="Developers have been running it for months." />
          </H2>
          <Small className="mt-5">From the beta program · Dec 2025 – Feb 2026</Small>
        </div>
        <div className="mt-16 grid gap-4 md:mt-20 lg:grid-cols-12">
          {QUOTES.map(([quote, author], index) => (
            <Reveal key={author} delay={index * 0.08} className={QUOTE_STYLE[index].split(" ")[0]}>
              <figure
                className={cn(
                  "m-0 flex h-full min-h-[300px] flex-col justify-between rounded-[22px] p-8 md:p-10",
                  QUOTE_STYLE[index].split(" ").slice(1).join(" ")
                )}
              >
                <blockquote className="m-0 font-display text-[32px]/[1.1] tracking-[-0.025em] md:text-[46px]/[1.06]">
                  “{quote}”
                </blockquote>
                <figcaption className="mt-8 text-[15px] opacity-75">{author}</figcaption>
              </figure>
            </Reveal>
          ))}
        </div>
        <ul
          className={cn(
            "m-0 mt-4 grid list-none grid-cols-2 overflow-hidden rounded-[22px] border p-0 lg:grid-cols-4",
            HAIRLINE
          )}
        >
          {STATS.map(stat => (
            <li
              key={stat.label || String(stat.value)}
              className="flex flex-col gap-4 border-site-light-080 px-7 py-8 not-last:border-r"
            >
              <stat.icon className="size-6 text-brand-strong" aria-hidden />
              <p className="m-0 font-display text-[40px]/[1] tracking-[-0.03em] text-fg md:text-[52px]/[1]">
                {stat.label === "releases" ? <LiveCount value={releases} /> : stat.value}
                {stat.label ? (
                  <span className="ml-2.5 align-baseline font-sans text-[18px] tracking-normal text-muted-foreground md:text-[20px]">
                    {stat.label}
                  </span>
                ) : null}
              </p>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}

export function Supervise() {
  return (
    <section id="supervise" className={cn("relative border-t py-24 md:py-44", HAIRLINE)}>
      <div className={CONTAINER}>
        <div className="mx-auto max-w-[900px] text-center">
          <Reveal>
            <LandingEyebrow>Supervise</LandingEyebrow>
          </Reveal>
          <H2 size="lg">
            <ScrollWords text="Details that matter." />
          </H2>
          <Reveal>
            <Lead className="mx-auto max-w-[760px]">
              Every session lands in Needs you, Working, Waiting, In review, or Done. Needs you is
              the only loud colour on the screen, so you know where to look.
            </Lead>
          </Reveal>
        </div>
        <Reveal className="mt-14 md:mt-20">
          <DetailPanels />
        </Reveal>
      </div>
    </section>
  );
}

const ORCHESTRATE_NOTES: ReadonlyArray<{ icon: Icon; text: string }> = [
  { icon: GitBranch, text: "Worktrees give parallel agents their own checkout of the same repo." },
  {
    icon: GitPullRequest,
    text: "The branch, the pull request, and its checks sit on the session.",
  },
  { icon: Workflow, text: "Continue a session with another agent CLI when one runs out." },
];

export function Orchestrate() {
  return (
    <section
      id="orchestrate"
      className={cn("relative overflow-x-clip border-t py-24 md:py-44", HAIRLINE)}
    >
      <Plate
        src={ASSET.plateBranch}
        className="top-0 right-0 h-[900px] w-[64%] opacity-90 [mask-image:linear-gradient(90deg,transparent,black_48%),linear-gradient(180deg,black_55%,transparent)] [mask-composite:intersect]"
      />
      <div className={cn(CONTAINER, "relative")}>
        <div className="max-w-[820px]">
          <Reveal>
            <LandingEyebrow icon={Workflow}>Orchestrate</LandingEyebrow>
          </Reveal>
          <H2 size="lg">
            <ScrollWords text="Agents hand work to each other, not to you." />
          </H2>
          <Reveal>
            <Lead className="max-w-[680px]">
              A session delegates to subagents and watches their progress live. An agent can end its
              turn and wake when a pull request's checks finish, another session settles, or a
              command exits.
            </Lead>
          </Reveal>
        </div>
        <Reveal className="mt-14 md:mt-20">
          <ProductWindow>
            <img
              src={ASSET.subagents}
              alt="A session with its three subagents open and running, and nested child sessions in the sidebar"
              draggable={false}
              className="block size-full object-cover"
            />
          </ProductWindow>
        </Reveal>
        <div className="mt-10 grid gap-8 md:grid-cols-3">
          {ORCHESTRATE_NOTES.map(note => (
            <Reveal key={note.text}>
              <note.icon className="size-5 text-brand-strong" aria-hidden />
              <p className="m-0 mt-3 text-[17px]/[1.4] text-fg/85">{note.text}</p>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}
