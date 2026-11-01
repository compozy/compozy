import { createElement, type ComponentType, type ReactNode, type SVGProps } from "react";

import { Bot, BrainCircuit, Code, Sparkles, Terminal, type LucideIcon } from "../../icons";
import { AntigravityLogo } from "../../logos/antigravity";
import { ClaudeLogo } from "../../logos/claude";
import { CursorLogo } from "../../logos/cursor";
import { GeminiLogo } from "../../logos/gemini";
import { KimiLogo } from "../../logos/kimi";
import { OpenAILogo } from "../../logos/openai";
import { OpenCodeLogo } from "../../logos/opencode";
import { OpenRouterLogo } from "../../logos/openrouter";
import { PiLogo } from "../../logos/pi";
import { QwenLogo } from "../../logos/qwen";
import { XAILogo } from "../../logos/xai";
import { ZAILogo } from "../../logos/zai";

type KindIconGlyphProps = SVGProps<SVGSVGElement>;
type KindIconRenderer = (props: KindIconGlyphProps) => ReactNode;

type KindIconRegistryEntry =
  | LucideIcon
  | {
      brand?: ComponentType<KindIconGlyphProps>;
      fallback?: LucideIcon;
      render?: KindIconRenderer;
    };

type KindIconRegistry<K extends string = string> = Record<K, KindIconRegistryEntry>;

/** No fixed brand ink: the mark follows the icon's text color, so it reads in either theme. */
function renderOpenAIKindLogo(props: KindIconGlyphProps) {
  return createElement(OpenAILogo, { ...props, mode: undefined });
}

const providerKindIconRegistry = {
  agy: { brand: AntigravityLogo, fallback: Sparkles },
  claude: { brand: ClaudeLogo, fallback: BrainCircuit },
  codex: { render: renderOpenAIKindLogo, fallback: Code },
  cursor: { brand: CursorLogo, fallback: Code },
  gemini: { brand: GeminiLogo, fallback: Sparkles },
  "kimi-cli": { brand: KimiLogo, fallback: Terminal },
  moonshot: { brand: KimiLogo, fallback: Sparkles },
  ollama: Terminal,
  openai: { render: renderOpenAIKindLogo, fallback: Bot },
  opencode: { brand: OpenCodeLogo, fallback: Terminal },
  openrouter: { brand: OpenRouterLogo, fallback: Sparkles },
  pi: { brand: PiLogo, fallback: BrainCircuit },
  "qwen-code": { brand: QwenLogo, fallback: Sparkles },
  xai: { brand: XAILogo, fallback: Sparkles },
  zai: { brand: ZAILogo, fallback: Sparkles },
} satisfies KindIconRegistry;

export { providerKindIconRegistry };
export type { KindIconRegistry, KindIconRegistryEntry };
