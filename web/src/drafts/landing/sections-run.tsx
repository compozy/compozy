import { cn } from "@compozy/ui";
import { GithubLogo, LinearLogo } from "@compozy/ui/logos";
import {
  Clock,
  Hand,
  Infinity as Bounded,
  Package,
  Repeat,
  Terminal as TerminalIcon,
} from "@compozy/ui/icons";
import { useInView, useReducedMotion } from "motion/react";
import { useEffect, useRef, useState, type ReactNode } from "react";

import context7 from "./assets/catalog/context7.png";
import notion from "./assets/catalog/notion.png";
import playwright from "./assets/catalog/playwright.png";
import postgres from "./assets/catalog/postgres.png";
import posthog from "./assets/catalog/posthog.svg";
import sentry from "./assets/catalog/sentry.svg";
import stripe from "./assets/catalog/stripe.svg";
import supabase from "./assets/catalog/supabase.png";
import automations from "./assets/s-automations.jpg";
import loopEditor from "./assets/s-dag.jpg";
import marketplace from "./assets/s-market.jpg";
import blocks from "./assets/plate-blocks.jpg";
import gates from "./assets/plate-gates.jpg";
import pulses from "./assets/plate-pulses.jpg";
import { Marquee, Reveal, ScrollWords } from "./motion";
import { CONTAINER, HAIRLINE } from "./layout";
import {
  LandingEyebrow,
  H2,
  Lead,
  Muted,
  Plate,
  PrimaryCta,
  SecondaryCta,
  Small,
  Shot,
  TextLink,
  type Icon,
} from "./primitives";

const LOOP_NOTES: ReadonlyArray<{ icon: Icon; text: string }> = [
  { icon: Bounded, text: "Bounded by design: every Loop carries an iteration cap." },
  {
    icon: Hand,
    text: "A human gate parks the run until someone approves, requests changes, or rejects.",
  },
  {
    icon: Repeat,
    text: "Seven named endings: done, no-op, blocked, failed, exhausted, stalled, canceled.",
  },
  { icon: TerminalIcon, text: "The same run from the web, the CLI, and HTTP." },
];

export function Loops() {
  return (
    <section
      id="loops"
      className={cn("relative overflow-x-clip border-t py-24 md:py-44", HAIRLINE)}
    >
      <Plate
        src={gates}
        className="top-0 right-[-8%] h-[820px] w-[75%] opacity-90 [mask-image:linear-gradient(90deg,transparent,black_35%,black_80%),linear-gradient(180deg,black_60%,transparent)] [mask-composite:intersect]"
      />
      <div className={cn(CONTAINER, "relative")}>
        <div className="max-w-[640px]">
          <Reveal>
            <LandingEyebrow icon={Repeat}>Loops</LandingEyebrow>
          </Reveal>
          <H2 size="lg">
            <ScrollWords text="Loops stop when the goal is verified." />
          </H2>
          <Reveal>
            <Lead>
              A Loop is a named contract: a goal, a way to verify it, and a stop. The daemon runs it
              unattended and always ends it honestly.
            </Lead>
          </Reveal>
        </div>
        <Reveal className="mt-16 md:mt-24">
          <Shot
            src={loopEditor}
            alt="The Loop editor: a graph of source, action and control nodes that branches on a route"
            aspect="aspect-[2/1]"
          />
        </Reveal>
        <ul className="m-0 mt-10 grid list-none gap-8 p-0 md:grid-cols-2 lg:grid-cols-4">
          {LOOP_NOTES.map(note => (
            <Reveal key={note.text}>
              <li>
                <note.icon className="size-5 text-brand-strong" aria-hidden />
                <span className="mt-3 block text-[16.5px]/[1.42] text-fg/85">{note.text}</span>
              </li>
            </Reveal>
          ))}
        </ul>
        <div className="mt-10 flex flex-wrap items-center justify-between gap-5">
          <Small>
            No Loops ship by default. Write one, fork one, or install an extension that ships one.
          </Small>
          <TextLink>Read the Loops guide</TextLink>
        </div>
      </div>
    </section>
  );
}

/** Types its text the first time it scrolls into view. */
export function Typed({ text, className }: { text: string; className?: string }) {
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref, { once: true, margin: "-15% 0px" });
  const reduced = useReducedMotion();
  const [count, setCount] = useState(0);
  useEffect(() => {
    if (!inView || reduced) return;
    const timer = window.setInterval(
      () => setCount(current => (current >= text.length ? current : current + 1)),
      24
    );
    return () => window.clearInterval(timer);
  }, [inView, reduced, text]);
  const shown = reduced ? text : text.slice(0, count);
  return (
    <span ref={ref} className={className}>
      <span className="sr-only">{text}</span>
      <span aria-hidden>
        {shown}
        <span className="ml-0.5 inline-block h-[1em] w-[0.55em] translate-y-[0.15em] bg-brand-strong [animation:landing-caret_1s_steps(2)_infinite] motion-reduce:[animation:none]" />
      </span>
    </span>
  );
}

export function Automations() {
  return (
    <section
      id="automations"
      className={cn("relative overflow-x-clip border-t bg-[#070707] pt-24 md:pt-44", HAIRLINE)}
    >
      <div className={cn(CONTAINER, "relative text-center")}>
        <Reveal>
          <LandingEyebrow icon={Clock}>Automations</LandingEyebrow>
        </Reveal>
        <H2 size="lg" className="mx-auto max-w-[1000px]">
          <ScrollWords text="Put agent work on a schedule or an event." />
        </H2>
        <Reveal>
          <Lead className="mx-auto max-w-[780px]">
            Schedules, runtime triggers, and signed webhooks live in one place. Every run is
            recorded, and finished runs leave your session list on their own.
          </Lead>
          <p className="m-0 mt-8 font-mono text-[16px] text-brand-strong md:text-[19px]">
            <Typed text="A briefing waiting for you every weekday at 08:30." />
          </p>
          <Muted className="mt-3">Manage the same job from the web, the CLI, or HTTP.</Muted>
        </Reveal>
      </div>
      <div className="relative mt-14 pb-24 md:mt-20 md:pb-40">
        <Plate
          src={pulses}
          className="inset-x-0 bottom-0 h-[760px] w-full [mask-image:linear-gradient(180deg,transparent,black_40%)]"
        />
        <Reveal className={cn(CONTAINER, "relative")}>
          <Shot
            src={automations}
            alt="A scheduled automation: when it starts, what it asks the agent, and its recorded runs"
          />
        </Reveal>
      </div>
    </section>
  );
}

const CATALOG_A: ReadonlyArray<readonly [string, ReactNode]> = [
  ["GitHub", <GithubLogo key="github" className="size-[22px]" />],
  ["Linear", <LinearLogo key="linear" className="size-[22px]" />],
  ["Context7", <CatalogMark key="context7" src={context7} />],
  ["Notion", <CatalogMark key="notion" src={notion} />],
  ["Sentry", <CatalogMark key="sentry" src={sentry} />],
];
const CATALOG_B: ReadonlyArray<readonly [string, ReactNode]> = [
  ["Playwright", <CatalogMark key="playwright" src={playwright} />],
  ["Stripe", <CatalogMark key="stripe" src={stripe} />],
  ["PostHog", <CatalogMark key="posthog" src={posthog} />],
  ["Postgres", <CatalogMark key="postgres" src={postgres} />],
  ["Supabase", <CatalogMark key="supabase" src={supabase} />],
];

function CatalogMark({ src }: { src: string }) {
  return (
    <img
      src={src}
      alt=""
      aria-hidden
      draggable={false}
      className="size-[22px] rounded-[5px] object-contain"
    />
  );
}

function CatalogRow({ items, reverse }: { items: typeof CATALOG_A; reverse?: boolean }) {
  return (
    <Marquee seconds={30} reverse={reverse}>
      {items.map(([name, mark]) => (
        <span
          key={name}
          className="inline-flex h-14 items-center gap-3 rounded-pill border border-[color:var(--light-100)] bg-[color:var(--light-040)] px-6 text-[16px] whitespace-nowrap text-fg backdrop-blur-md"
        >
          {mark}
          {name}
        </span>
      ))}
    </Marquee>
  );
}

export function Extensibility() {
  return (
    <section
      id="extensibility"
      className={cn("relative overflow-x-clip border-t py-24 md:py-44", HAIRLINE)}
    >
      <Plate
        src={blocks}
        className="top-0 right-[-6%] h-[860px] w-[62%] opacity-90 [mask-image:linear-gradient(90deg,transparent,black_38%),linear-gradient(180deg,black_55%,transparent)] [mask-composite:intersect]"
      />
      <div className={cn(CONTAINER, "relative")}>
        <div className="max-w-[820px]">
          <Reveal>
            <LandingEyebrow icon={Package}>Built to be built on</LandingEyebrow>
          </Reveal>
          <H2 size="lg">
            <ScrollWords text="One install. The daemon already has a place for it." />
          </H2>
          <Reveal>
            <Lead className="max-w-[740px]">
              Extensions, plugins, and MCP servers share one Marketplace, by category, with one-step
              Add. An extension brings skills, agents, tools, hooks, and check-in sources into the
              same registries your agents already use.
            </Lead>
            <p className="m-0 mt-6 font-mono text-[14px] text-fg/70">
              Write one in Go or TypeScript, with React for views: init, build, validate, dev,
              publish.
            </p>
          </Reveal>
        </div>
      </div>
      <div className="relative mt-14 md:mt-20">
        <div className={CONTAINER}>
          <Reveal>
            <Shot src={marketplace} alt="The Marketplace by category, with one-step Add" />
          </Reveal>
        </div>
        <div className="relative z-10 mt-12 space-y-3">
          <p className={cn(CONTAINER, "my-0 mb-2 font-mono text-[13px] text-fg/70")}>
            From the catalog
          </p>
          <CatalogRow items={CATALOG_A} />
          <CatalogRow items={CATALOG_B} reverse />
        </div>
      </div>
      <div className={cn(CONTAINER, "mt-14 flex flex-wrap gap-3")}>
        <PrimaryCta>Browse the marketplace</PrimaryCta>
        <SecondaryCta>Build an extension</SecondaryCta>
      </div>
    </section>
  );
}
