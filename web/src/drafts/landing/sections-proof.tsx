import { cn } from "@compozy/ui";
import {
  Bookmark,
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
import { m, useReducedMotion } from "motion/react";
import { useEffect, useState } from "react";

import cropApproval from "./assets/crop-approval.jpg";
import cropLater from "./assets/crop-later.jpg";
import cropReview from "./assets/crop-review.jpg";
import branch from "./assets/plate-branch.jpg";
import subagents from "./assets/s-subagents.jpg";
import brief from "./assets/uc-brief.jpg";
import claims from "./assets/uc-claims.jpg";
import gate from "./assets/uc-gate.jpg";
import review from "./assets/uc-review.jpg";
import spec from "./assets/uc-spec.jpg";
import triage from "./assets/uc-triage.jpg";
import { LIVE, QUOTES } from "./content";
import { CountUp, Reveal, ScrollWords } from "./motion";
import { CONTAINER, HAIRLINE } from "./layout";
import {
  LandingEyebrow,
  H2,
  H3,
  Lead,
  Muted,
  Plate,
  ProductWindow,
  Small,
  type Icon,
} from "./primitives";

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
    art: triage,
    span: "lg:col-span-7",
  },
  {
    lane: "Dev",
    icon: GitBranch,
    title: "Deliver a spec while you are away",
    body: "A conductor splits the work into slices. Each slice runs in its own worktree.",
    art: spec,
    span: "lg:col-span-5",
  },
  {
    lane: "Dev",
    icon: Search,
    title: "Review a change until the round is clean",
    body: "A reviewer finds, an implementer fixes. Findings land in a file you can open.",
    art: review,
    span: "lg:col-span-5",
  },
  {
    lane: "Ops",
    icon: CalendarClock,
    title: "Leave a briefing waiting every weekday",
    body: "A schedule starts an agent with nobody watching. Every run is recorded, with retry on failure.",
    art: brief,
    span: "lg:col-span-7",
  },
  {
    lane: "Ops",
    icon: ListChecks,
    title: "Keep release checks as claimed work",
    body: "Each concern is a task an agent claims. Runs close with evidence.",
    art: claims,
    span: "lg:col-span-7",
  },
  {
    lane: "Ops",
    icon: Hand,
    title: "Wait for a person before a sensitive action",
    body: "A tool call can require approval before it runs. Waiting work rises to Needs you.",
    art: gate,
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
              <article className="group relative h-[420px] overflow-hidden rounded-[22px] border border-[color:var(--light-090)] bg-black">
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
                  <p className="m-0 inline-flex w-fit items-center gap-2 rounded-pill border border-[color:var(--light-150)] bg-[color:var(--dark-450)] py-1.5 pr-3.5 pl-2.5 font-mono text-[12.5px] text-fg backdrop-blur-md">
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
  "lg:col-span-12 border border-[color:var(--light-100)] bg-[#111] text-fg",
];

const STATS: ReadonlyArray<{ icon: Icon; value: number | string; label: string }> = [
  { icon: Star, value: LIVE.stars, label: "on GitHub" },
  { icon: Scale, value: "MIT", label: "license" },
  { icon: Package, value: LIVE.releases, label: "releases" },
  { icon: Monitor, value: "Local-first", label: "" },
];

export function Community() {
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
              key={stat.value}
              className="flex flex-col gap-4 border-[color:var(--light-080)] px-7 py-8 not-last:border-r"
            >
              <stat.icon className="size-6 text-brand-strong" aria-hidden />
              <p className="m-0 font-display text-[40px]/[1] tracking-[-0.03em] text-fg md:text-[52px]/[1]">
                {typeof stat.value === "number" ? <CountUp value={stat.value} /> : stat.value}
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

const DETAIL_PANELS: ReadonlyArray<{
  icon: Icon;
  src: string;
  art: string;
  alt: string;
  text: string;
}> = [
  {
    icon: Hand,
    src: cropApproval,
    art: gate,
    alt: "A tool call waiting for Don't allow, Always allow or Allow once",
    text: "Approve a tool call in one click, from the session or the bell.",
  },
  {
    icon: Bookmark,
    src: cropLater,
    art: brief,
    alt: "The Move to Later dialog with a reason and a reminder date",
    text: "Park a session for later, with a reason and a date.",
  },
  {
    icon: GitPullRequest,
    src: cropReview,
    art: branch,
    alt: "The In review column with each session's pull request",
    text: "A pull request moves its session to In review on its own.",
  },
];
const DETAIL_STEP_MS = 5200;

/** Three details as panels that open in turn; pointing at one takes over from the timer. */
function DetailPanels() {
  const reduced = useReducedMotion();
  const [active, setActive] = useState(0);
  const [held, setHeld] = useState(false);
  useEffect(() => {
    if (reduced || held) return;
    const timer = window.setInterval(
      () => setActive(current => (current + 1) % DETAIL_PANELS.length),
      DETAIL_STEP_MS
    );
    return () => window.clearInterval(timer);
  }, [reduced, held]);
  return (
    <div
      className="flex flex-col gap-4 lg:h-[640px] lg:flex-row"
      onPointerLeave={() => setHeld(false)}
    >
      {DETAIL_PANELS.map((panel, index) => {
        const on = index === active;
        return (
          <button
            key={panel.text}
            type="button"
            aria-pressed={on}
            onPointerEnter={() => {
              setActive(index);
              setHeld(true);
            }}
            onFocus={() => {
              setActive(index);
              setHeld(true);
            }}
            onClick={() => setActive(index)}
            className={cn(
              "group relative h-[520px] min-w-0 basis-0 cursor-pointer overflow-hidden rounded-[26px] border bg-black text-left transition-[flex-grow,border-color] duration-700 ease-[cubic-bezier(0.16,1,0.3,1)] focus-visible:outline-none focus-visible:shadow-focus-ring lg:h-auto",
              on
                ? "border-[color:var(--light-250)] lg:grow-[2.7]"
                : "border-[color:var(--light-100)] lg:grow"
            )}
          >
            <img
              src={panel.art}
              alt=""
              aria-hidden
              draggable={false}
              className={cn(
                "absolute inset-0 size-full object-cover transition-opacity duration-700",
                on ? "opacity-70" : "opacity-40"
              )}
            />
            <div
              aria-hidden
              className="absolute inset-0 bg-[linear-gradient(180deg,rgba(10,10,10,0.1)_40%,rgba(10,10,10,0.94)_84%)]"
            />
            <div className="absolute inset-x-6 top-6 bottom-[150px] flex items-center justify-center">
              <img
                src={panel.src}
                alt={panel.alt}
                draggable={false}
                className={cn(
                  "max-h-full max-w-full rounded-[14px] border border-[color:var(--light-150)] object-contain shadow-[0_30px_80px_-10px_#000] transition-[opacity,transform] duration-700 ease-[cubic-bezier(0.16,1,0.3,1)]",
                  on ? "scale-100 opacity-100" : "scale-[0.92] opacity-30"
                )}
              />
            </div>
            <div className="absolute inset-x-7 bottom-7">
              <panel.icon
                className={cn("size-6 transition-colors", on ? "text-brand-strong" : "text-fg/60")}
                aria-hidden
              />
              <p className="m-0 mt-3 max-w-[520px] font-display text-[21px]/[1.2] tracking-[-0.015em] text-fg md:text-[25px]/[1.18]">
                {panel.text}
              </p>
            </div>
            {on && !held && !reduced ? (
              <m.span
                key={active}
                aria-hidden
                className="absolute inset-x-0 bottom-0 h-[2px] origin-left bg-[linear-gradient(90deg,#1f87fc,#1df0d7)]"
                initial={{ scaleX: 0 }}
                animate={{ scaleX: 1 }}
                transition={{ duration: DETAIL_STEP_MS / 1000, ease: "linear" }}
              />
            ) : null}
          </button>
        );
      })}
    </div>
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
        src={branch}
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
              src={subagents}
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
