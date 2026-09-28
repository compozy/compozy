import { Bot, BrainCircuit, Code, Sparkles, Terminal, type LucideIcon } from "lucide-react";
import { createElement, type ComponentType, type ReactNode, type SVGProps } from "react";

import { BlackboxLogo } from "../../logos/blackbox";
import { ClaudeLogo } from "../../logos/claude";
import { ClineLogo } from "../../logos/cline";
import { CursorLogo } from "../../logos/cursor";
import { GeminiLogo } from "../../logos/gemini";
import { GooseLogo } from "../../logos/goose";
import { GroqLogo } from "../../logos/groq";
import { HermesLogo } from "../../logos/hermes";
import { JunieLogo } from "../../logos/junie";
import { KimiLogo } from "../../logos/kimi";
import { KiroLogo } from "../../logos/kiro";
import { MinimaxLogo } from "../../logos/minimax";
import { MistralLogo } from "../../logos/mistral";
import { OpenAILogo } from "../../logos/openai";
import { OpenClawLogo } from "../../logos/openclaw";
import { OpenCodeLogo } from "../../logos/opencode";
import { OpenHandsLogo } from "../../logos/openhands";
import { OpenRouterLogo } from "../../logos/openrouter";
import { PiLogo } from "../../logos/pi";
import { QoderLogo } from "../../logos/qoder";
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

function renderOpenAIKindLogo(props: KindIconGlyphProps) {
  return createElement(OpenAILogo, { ...props, mode: "dark" });
}

const providerKindIconRegistry = {
  blackbox: { brand: BlackboxLogo, fallback: Bot },
  claude: { brand: ClaudeLogo, fallback: BrainCircuit },
  cline: { brand: ClineLogo, fallback: Code },
  codex: { render: renderOpenAIKindLogo, fallback: Code },
  cursor: { brand: CursorLogo, fallback: Code },
  gemini: { brand: GeminiLogo, fallback: Sparkles },
  goose: { brand: GooseLogo, fallback: Terminal },
  groq: { brand: GroqLogo, fallback: Sparkles },
  hermes: { brand: HermesLogo, fallback: BrainCircuit },
  junie: { brand: JunieLogo, fallback: Sparkles },
  "kimi-cli": { brand: KimiLogo, fallback: Terminal },
  kiro: { brand: KiroLogo, fallback: Terminal },
  minimax: { brand: MinimaxLogo, fallback: Sparkles },
  mistral: { brand: MistralLogo, fallback: Sparkles },
  moonshot: { brand: KimiLogo, fallback: Sparkles },
  ollama: Terminal,
  openai: { render: renderOpenAIKindLogo, fallback: Bot },
  openclaw: { brand: OpenClawLogo, fallback: Bot },
  opencode: { brand: OpenCodeLogo, fallback: Terminal },
  openhands: { brand: OpenHandsLogo, fallback: Code },
  openrouter: { brand: OpenRouterLogo, fallback: Sparkles },
  pi: { brand: PiLogo, fallback: BrainCircuit },
  qoder: { brand: QoderLogo, fallback: Code },
  "qwen-code": { brand: QwenLogo, fallback: Sparkles },
  xai: { brand: XAILogo, fallback: Sparkles },
  zai: { brand: ZAILogo, fallback: Sparkles },
} satisfies KindIconRegistry;

export { providerKindIconRegistry };
export type { KindIconRegistry, KindIconRegistryEntry };
