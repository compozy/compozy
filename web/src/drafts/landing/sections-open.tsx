import { Logo, cn } from "@compozy/ui";
import { Download, Star } from "@compozy/ui/icons";
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
import { m, useMotionValueEvent, useReducedMotion, useScroll, useSpring } from "motion/react";
import { useEffect, useState, type ComponentType, type SVGProps } from "react";

import awayFull from "./assets/s-hero-away.jpg";
import board from "./assets/board.jpg";
import clarify from "./assets/s-clarify.jpg";
import heroPlate from "./assets/plate-hero.jpg";
import { Dust } from "./fx/dust";
import { INSTALL_COMMAND, LIVE, NAV } from "./content";
import { CountUp, Marquee, Reveal, WordsIn } from "./motion";
import { useHeroMotion } from "./hooks/use-hero-motion";
import { CONTAINER, HAIRLINE, type Focus } from "./layout";
import {
  Camera,
  InstallCommand,
  LandingEyebrow,
  H1,
  H2,
  Lead,
  Muted,
  Plate,
  PrimaryCta,
  ProductWindow,
  Small,
  TextLink,
} from "./primitives";

export function SiteHeader() {
  const { scrollY, scrollYProgress } = useScroll();
  const [compact, setCompact] = useState(false);
  useMotionValueEvent(scrollY, "change", value => setCompact(value > 80));
  const width = useSpring(scrollYProgress, { stiffness: 120, damping: 30 });
  return (
    <header className="fixed inset-x-0 top-0 z-50 flex justify-center px-4 pt-3">
      <m.div
        className={cn(
          "relative flex h-14 w-full items-center justify-between overflow-hidden rounded-[18px] border pr-2.5 pl-5 transition-[max-width,background-color,border-color,backdrop-filter] duration-500",
          compact
            ? "max-w-[920px] border-[color:var(--light-120)] bg-[#101010]/75 backdrop-blur-2xl"
            : "max-w-[1280px] border-transparent bg-transparent"
        )}
      >
        <Logo variant="logo" className="h-[22px]" />
        <nav className="hidden gap-7 md:flex">
          {NAV.map(item => (
            <a
              key={item}
              href="#"
              className="text-[14px] text-fg/70 transition-colors hover:text-fg"
            >
              {item}
            </a>
          ))}
        </nav>
        <a
          href="#"
          className="inline-flex h-9 items-center gap-1.5 rounded-[11px] border border-[color:var(--light-150)] bg-[color:var(--light-050)] px-3.5 text-[13.5px] text-fg transition-colors hover:bg-[color:var(--light-100)]"
        >
          <Star className="size-3.5" aria-hidden /> Star on GitHub
        </a>
        <m.span
          aria-hidden
          style={{ scaleX: width }}
          className={cn(
            "absolute inset-x-0 bottom-0 h-px origin-left bg-[linear-gradient(90deg,#1f87fc,#1df0d7)] transition-opacity",
            compact ? "opacity-100" : "opacity-0"
          )}
        />
      </m.div>
    </header>
  );
}

const WHOLE_FRAME: Focus = { x: 0, y: 0, w: 1 };
const DRIFT: Focus = { x: 0.012, y: 0.012, w: 0.976 };
const CLIP: ReadonlyArray<{ src: string; alt: string; from: Focus; to: Focus }> = [
  {
    src: awayFull,
    alt: "An always-on agent's main session with its While you were away card",
    from: WHOLE_FRAME,
    to: DRIFT,
  },
  {
    src: clarify,
    alt: "The sessions board with a session open beside it, asking a person to choose",
    from: WHOLE_FRAME,
    to: DRIFT,
  },
  {
    src: board,
    alt: "The sessions board, sorted by who owns the next step",
    from: WHOLE_FRAME,
    to: DRIFT,
  },
];
const STEP_MS = 5600;
const POSTER = CLIP.length - 1;

/** Stands in for the recorded hero clip: real frames, filmed in the order the clip will follow. */
function HeroClip() {
  const reduced = useReducedMotion();
  const [frame, setFrame] = useState(reduced ? POSTER : 0);
  const [settled, setSettled] = useState(false);
  useEffect(() => {
    if (reduced) return;
    const settle = window.setTimeout(() => setSettled(true), 500);
    const timer = window.setInterval(() => {
      setSettled(false);
      setFrame(current => (current + 1) % CLIP.length);
      window.setTimeout(() => setSettled(true), 500);
    }, STEP_MS);
    return () => {
      window.clearTimeout(settle);
      window.clearInterval(timer);
    };
  }, [reduced]);
  return (
    <>
      {CLIP.map((item, index) => (
        <Camera
          key={item.src}
          src={item.src}
          alt={item.alt}
          active={index === frame}
          focus={index === frame && (settled || reduced) ? item.to : item.from}
          duration={3.4}
        />
      ))}
      <div aria-hidden className="absolute inset-x-5 bottom-4 z-10 flex gap-1.5">
        {CLIP.map((item, index) => (
          <span
            key={item.src}
            className="h-[3px] flex-1 overflow-hidden rounded-pill bg-[color:var(--light-150)]"
          >
            {index === frame && !reduced ? (
              <m.span
                key={frame}
                className="block h-full origin-left bg-brand"
                initial={{ scaleX: 0 }}
                animate={{ scaleX: 1 }}
                transition={{ duration: STEP_MS / 1000, ease: "linear" }}
              />
            ) : null}
          </span>
        ))}
      </div>
    </>
  );
}

export function Hero() {
  const { ref, reduced, desktop, rotateX, scale, plateY, copyY, copyOpacity } = useHeroMotion();
  return (
    <section ref={ref} id="hero" className="relative overflow-x-clip pb-20 md:pb-32">
      <Plate
        src={heroPlate}
        style={{ y: plateY }}
        className="inset-x-0 top-0 h-[1120px] w-full [mask-image:linear-gradient(180deg,black_62%,transparent)]"
      />
      {desktop && !reduced ? (
        <Dust
          count={170}
          className="absolute inset-x-0 top-0 h-[1000px] w-full [mask-image:linear-gradient(180deg,black_60%,transparent)]"
        />
      ) : null}
      <m.div
        style={{ y: copyY, opacity: copyOpacity }}
        className={cn(CONTAINER, "relative z-10 pt-36 text-center md:pt-44")}
      >
        <Reveal y={10}>
          <LandingEyebrow>An operating system for AI agents</LandingEyebrow>
        </Reveal>
        <div className="mx-auto max-w-[1080px]">
          <H1>
            <WordsIn text="The autonomous control plane for your agents." delay={0.1} />
          </H1>
        </div>
        <Reveal delay={0.55} y={16}>
          <Lead className="mx-auto max-w-[720px]">
            See and steer every Claude Code, Codex, and Cursor session. Always-on agents check in on
            their own, and you step in only when one needs you.
          </Lead>
        </Reveal>
        <Reveal delay={0.75} y={16}>
          <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
            <PrimaryCta icon={Download}>Download for macOS</PrimaryCta>
            <InstallCommand command={INSTALL_COMMAND} />
          </div>
          <div className="mt-6 flex flex-col items-center gap-3">
            <TextLink>Open the runtime docs</TextLink>
            <Small className="text-fg/45">
              Verified installer · macOS and Linux &nbsp;·&nbsp; Desktop for macOS and Linux · CLI
              for macOS, Linux, and Windows
            </Small>
          </div>
        </Reveal>
      </m.div>
      <div className="relative z-20 mx-auto mt-14 w-full max-w-[1560px] px-4 [perspective:1500px] md:mt-20 md:px-8">
        <ProductWindow style={{ rotateX, scale, transformOrigin: "50% 0%" }}>
          <HeroClip />
        </ProductWindow>
      </div>
    </section>
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
          className="flex h-[84px] items-center gap-3.5 rounded-[18px] border border-[color:var(--light-090)] bg-[color:var(--light-030)] pr-7 pl-5 backdrop-blur-md"
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
            <CountUp value={LIVE.providers} />
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
        <TextLink>See supported agents</TextLink>
      </div>
    </section>
  );
}
