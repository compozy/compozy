"use client";

/**
 * The two rows of agent logos. A client component on purpose: `Marquee` renders its row twice, and
 * logos with SVG defs (Gemini) take their ids from `useId`, so each copy has to render here to get
 * its own ids; a server-rendered row would be repeated with the same ones.
 */
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

import { Marquee } from "@/components/site/motion";

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

export function ProviderMarquees() {
  return (
    <div className="mt-16 space-y-3 md:mt-20">
      <ProviderRow items={ROW_ONE} seconds={38} />
      <ProviderRow items={ROW_TWO} seconds={46} reverse />
    </div>
  );
}
