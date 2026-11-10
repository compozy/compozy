/** Running work: Loops, automations, and the extension ecosystem. */
import { cn } from "@compozy/ui";
import {
  Clock,
  Hand,
  Infinity as Bounded,
  Package,
  Repeat,
  Terminal as TerminalIcon,
} from "@compozy/ui/icons";
import { GithubLogo, LinearLogo } from "@compozy/ui/logos";
import type { ReactNode } from "react";

import { ASSET } from "@/components/landing/assets";
import { HREF } from "@/components/landing/content";
import { Typed } from "@/components/landing/typed";
import { CONTAINER, HAIRLINE } from "@/components/site/layout";
import { Marquee, Reveal, ScrollWords } from "@/components/site/motion";
import {
  H2,
  LandingEyebrow,
  Lead,
  Muted,
  Plate,
  PrimaryCta,
  SecondaryCta,
  Shot,
  Small,
  TextLink,
  type Icon,
} from "@/components/site/primitives";

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
        src={ASSET.plateGates}
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
            src={ASSET.loopEditor}
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
          <TextLink href={HREF.loops}>Read the Loops guide</TextLink>
        </div>
      </div>
    </section>
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
          src={ASSET.platePulses}
          className="inset-x-0 bottom-0 h-[760px] w-full [mask-image:linear-gradient(180deg,transparent,black_40%)]"
        />
        <Reveal className={cn(CONTAINER, "relative")}>
          <Shot
            src={ASSET.automations}
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
  ["Context7", <CatalogMark key="context7" src={ASSET.catalog.context7} />],
  ["Notion", <CatalogMark key="notion" src={ASSET.catalog.notion} />],
  ["Sentry", <CatalogMark key="sentry" src={ASSET.catalog.sentry} />],
];
const CATALOG_B: ReadonlyArray<readonly [string, ReactNode]> = [
  ["Playwright", <CatalogMark key="playwright" src={ASSET.catalog.playwright} />],
  ["Stripe", <CatalogMark key="stripe" src={ASSET.catalog.stripe} />],
  ["PostHog", <CatalogMark key="posthog" src={ASSET.catalog.posthog} />],
  ["Postgres", <CatalogMark key="postgres" src={ASSET.catalog.postgres} />],
  ["Supabase", <CatalogMark key="supabase" src={ASSET.catalog.supabase} />],
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
          className="inline-flex h-14 items-center gap-3 rounded-pill border border-site-light-100 bg-site-light-040 px-6 text-[16px] whitespace-nowrap text-fg backdrop-blur-md"
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
        src={ASSET.plateBlocks}
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
            <Shot src={ASSET.marketplace} alt="The Marketplace by category, with one-step Add" />
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
        <PrimaryCta href={HREF.marketplace}>Browse the marketplace</PrimaryCta>
        <SecondaryCta href={HREF.buildExtension}>Build an extension</SecondaryCta>
      </div>
    </section>
  );
}
