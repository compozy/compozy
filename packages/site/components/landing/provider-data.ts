export const SUPPORTED_AGENT_PROVIDERS = [
  { id: "claude", name: "Claude Code" },
  { id: "codex", name: "Codex" },
  { id: "gemini", name: "Gemini CLI" },
  { id: "opencode", name: "OpenCode" },
  { id: "copilot", name: "GitHub Copilot CLI" },
  { id: "cursor", name: "Cursor Agent" },
  { id: "kiro", name: "Kiro CLI" },
  { id: "pi", name: "Pi" },
  { id: "blackbox", name: "BLACKBOX AI" },
  { id: "cline", name: "Cline" },
  { id: "goose", name: "Goose" },
  { id: "hermes", name: "Hermes" },
  { id: "junie", name: "Junie" },
  { id: "kimi-cli", name: "Kimi CLI" },
  { id: "openclaw", name: "OpenClaw" },
  { id: "openhands", name: "OpenHands" },
  { id: "qoder", name: "Qoder CLI" },
  { id: "qwen-code", name: "Qwen Code" },
  { id: "openrouter", name: "OpenRouter" },
  { id: "zai", name: "z.ai" },
  { id: "moonshot", name: "Moonshot / Kimi" },
  { id: "vercel-ai-gateway", name: "Vercel AI Gateway" },
  { id: "xai", name: "xAI" },
  { id: "minimax", name: "MiniMax" },
  { id: "mistral", name: "Mistral" },
  { id: "groq", name: "Groq" },
] as const;

export type SupportedAgentProvider = (typeof SUPPORTED_AGENT_PROVIDERS)[number];

export const SUPPORTED_AGENT_COUNT = SUPPORTED_AGENT_PROVIDERS.length;
