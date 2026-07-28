import {
  Bot,
  BrainCircuit,
  Code,
  GitBranch,
  MessageSquare,
  Send,
  Sparkles,
  SquareKanban,
  Terminal,
  Users,
  type LucideIcon,
} from "lucide-react";
import { createElement, type ComponentType, type ReactNode, type SVGProps } from "react";

import { BlackboxLogo } from "../../logos/blackbox";
import { ClaudeLogo } from "../../logos/claude";
import { ClineLogo } from "../../logos/cline";
import { CursorLogo } from "../../logos/cursor";
import { DiscordLogo } from "../../logos/discord";
import { GeminiLogo } from "../../logos/gemini";
import { GithubLogo } from "../../logos/github";
import { GooseLogo } from "../../logos/goose";
import { GoogleChatLogo } from "../../logos/google-chat";
import { GroqLogo } from "../../logos/groq";
import { HermesLogo } from "../../logos/hermes";
import { JunieLogo } from "../../logos/junie";
import { KimiLogo } from "../../logos/kimi";
import { KiroLogo } from "../../logos/kiro";
import { LinearLogo } from "../../logos/linear";
import { MicrosoftTeamsLogo } from "../../logos/microsoft-teams";
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
import { SlackLogo } from "../../logos/slack";
import { TelegramLogo } from "../../logos/telegram";
import { WhatsAppLogo } from "../../logos/whatsapp";
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

function renderLinearKindLogo(props: KindIconGlyphProps) {
  return createElement(LinearLogo, { ...props, mode: "dark" });
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

const bridgeKindIconRegistry = {
  discord: { brand: DiscordLogo, fallback: MessageSquare },
  github: { brand: GithubLogo, fallback: GitBranch },
  "google-chat": { brand: GoogleChatLogo, fallback: MessageSquare },
  google_chat: { brand: GoogleChatLogo, fallback: MessageSquare },
  linear: { render: renderLinearKindLogo, fallback: SquareKanban },
  "microsoft-teams": { brand: MicrosoftTeamsLogo, fallback: Users },
  microsoft_teams: { brand: MicrosoftTeamsLogo, fallback: Users },
  slack: { brand: SlackLogo, fallback: MessageSquare },
  telegram: { brand: TelegramLogo, fallback: Send },
  whatsapp: { brand: WhatsAppLogo, fallback: MessageSquare },
} satisfies KindIconRegistry;

export { bridgeKindIconRegistry, providerKindIconRegistry };
export type { KindIconRegistry, KindIconRegistryEntry };
