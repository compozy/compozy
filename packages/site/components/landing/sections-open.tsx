/** Hero and the agents CompozyOS runs. */
import { cn } from "@compozy/ui";
import { Download } from "@compozy/ui/icons";

import {
  HERO_CATEGORY,
  HERO_HEADLINE,
  HERO_SUBHEAD,
  HREF,
  INSTALL_COMMAND,
} from "@/components/landing/content";
import { HeroScene } from "@/components/landing/hero-scene";
import { ProviderMarquees } from "@/components/landing/provider-marquees";
import { BUILTIN_PROVIDER_COUNT } from "@/components/landing/provider-data";
import { CONTAINER, HAIRLINE } from "@/components/site/layout";
import { CountUp, Entrance, Reveal, WordsIn } from "@/components/site/motion";
import {
  H1,
  H2,
  InstallCommand,
  LandingEyebrow,
  Lead,
  Muted,
  PrimaryCta,
  Small,
  TextLink,
} from "@/components/site/primitives";

export function Hero() {
  return (
    <HeroScene>
      <Entrance y={10}>
        <LandingEyebrow>{HERO_CATEGORY}</LandingEyebrow>
      </Entrance>
      <div className="mx-auto max-w-[1080px]">
        <H1>
          <WordsIn text={HERO_HEADLINE} delay={0.1} />
        </H1>
      </div>
      <Entrance delay={0.55} y={16}>
        <Lead className="mx-auto max-w-[720px]">{HERO_SUBHEAD}</Lead>
      </Entrance>
      <Entrance delay={0.75} y={16}>
        <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
          <PrimaryCta icon={Download} href={HREF.download}>
            Download for macOS
          </PrimaryCta>
          <InstallCommand command={INSTALL_COMMAND} />
        </div>
        <div className="mt-6 flex flex-col items-center gap-3">
          <TextLink href={HREF.docs}>Open the runtime docs</TextLink>
          <Small className="text-fg/45">
            Verified installer · macOS and Linux &nbsp;·&nbsp; Desktop for macOS and Linux · CLI for
            macOS, Linux, and Windows
          </Small>
        </div>
      </Entrance>
    </HeroScene>
  );
}

export function Providers() {
  return (
    <section id="providers" className={cn("relative border-t py-24 md:py-40", HAIRLINE)}>
      <div className={cn(CONTAINER, "grid items-end gap-10 lg:grid-cols-[1.25fr_1fr] lg:gap-24")}>
        <Reveal>
          <H2 size="md">Runs the agents you already use.</H2>
          <Lead className="max-w-[620px]">
            CompozyOS starts the agent CLIs you already have as durable sessions you can see, steer,
            and hand off.
          </Lead>
        </Reveal>
        <Reveal delay={0.15}>
          <p className="m-0 font-display text-[88px] leading-none tracking-[-0.04em] text-fg md:text-[128px]">
            <CountUp value={BUILTIN_PROVIDER_COUNT} />
          </p>
          <p className="m-0 mt-2 font-mono text-[13px] text-muted-foreground">
            built-in integrations
          </p>
        </Reveal>
      </div>
      <ProviderMarquees />
      <div className={cn(CONTAINER, "mt-12 flex flex-wrap items-center justify-between gap-5")}>
        <Muted>
          Your subscriptions and API keys stay yours. CompozyOS resells no model access.
        </Muted>
        <TextLink href={HREF.providers}>See supported agents</TextLink>
      </div>
    </section>
  );
}
