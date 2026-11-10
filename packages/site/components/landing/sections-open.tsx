/** Hero and the agents CompozyOS runs. */
import { cn } from "@compozy/ui";
import { Download } from "@compozy/ui/icons";
import {
  ClaudeLogo,
  CopilotLogo,
  CursorLogo,
  GeminiLogo,
  KimiLogo,
  OpenAILogo,
  OpenCodeLogo,
  OpenRouterLogo,
  QwenLogo,
  VercelLogo,
  XAILogo,
} from "@compozy/ui/logos";
import type { ComponentType, SVGProps } from "react";

import {
  HERO_CATEGORY,
  HERO_HEADLINE,
  HERO_SUBHEAD,
  HREF,
  INSTALL_COMMAND,
} from "@/components/landing/content";
import { HeroScene } from "@/components/landing/hero-scene";
import { BUILTIN_PROVIDER_COUNT } from "@/components/landing/provider-data";
import { CONTAINER, HAIRLINE } from "@/components/site/layout";
import { CountUp, Reveal, WordsIn } from "@/components/landing/motion";
import { Marquee } from "@/components/site/motion";
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
      <Reveal y={10}>
        <LandingEyebrow>{HERO_CATEGORY}</LandingEyebrow>
      </Reveal>
      <div className="mx-auto max-w-[1080px]">
        <H1>
          <WordsIn text={HERO_HEADLINE} delay={0.1} />
        </H1>
      </div>
      <Reveal delay={0.55} y={16}>
        <Lead className="mx-auto max-w-[720px]">{HERO_SUBHEAD}</Lead>
      </Reveal>
      <Reveal delay={0.75} y={16}>
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
      </Reveal>
    </HeroScene>
  );
}

function WordmarkGlyph({ className }: SVGProps<SVGSVGElement>) {
  return <OpenCodeLogo className={className} viewBox="0 0 30 42" />;
}
function CodexGlyph({ className }: SVGProps<SVGSVGElement>) {
  return <OpenAILogo className={className} />;
}

type Mark = ComponentType<SVGProps<SVGSVGElement>>;
const ROW_ONE: ReadonlyArray<readonly [string, Mark]> = [
  ["Claude Code", ClaudeLogo],
  ["Codex", CodexGlyph],
  ["Cursor", CursorLogo],
  ["Gemini", GeminiLogo],
  ["OpenCode", WordmarkGlyph],
  ["GitHub Copilot", CopilotLogo],
];
const ROW_TWO: ReadonlyArray<readonly [string, Mark]> = [
  ["Qwen Code", QwenLogo],
  ["Kimi", KimiLogo],
  ["OpenRouter", OpenRouterLogo],
  ["Moonshot", KimiLogo],
  ["xAI", XAILogo],
  ["Vercel AI Gateway", VercelLogo],
];

function ProviderRow({
  items,
  reverse,
  seconds,
}: {
  items: typeof ROW_ONE;
  reverse?: boolean;
  seconds: number;
}) {
  return (
    <Marquee reverse={reverse} seconds={seconds}>
      {items.map(([name, Glyph]) => (
        <span
          key={name}
          className="flex h-[84px] items-center gap-3.5 rounded-[18px] border border-site-light-090 bg-site-light-030 pr-7 pl-5 backdrop-blur-md"
        >
          <Glyph className="size-7 shrink-0" aria-hidden />
          <span className="text-[17px] whitespace-nowrap text-fg">{name}</span>
        </span>
      ))}
    </Marquee>
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
      <div className="mt-16 space-y-3 md:mt-20">
        <ProviderRow items={ROW_ONE} seconds={38} />
        <ProviderRow items={ROW_TWO} seconds={46} reverse />
      </div>
      <div className={cn(CONTAINER, "mt-12 flex flex-wrap items-center justify-between gap-5")}>
        <Muted>
          Your subscriptions and API keys stay yours. CompozyOS resells no model access.
        </Muted>
        <TextLink href={HREF.providers}>See supported agents</TextLink>
      </div>
    </section>
  );
}
