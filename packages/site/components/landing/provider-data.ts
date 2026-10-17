export const BUILTIN_PROVIDER_INTEGRATIONS = [
  { id: "claude", name: "Claude Code" },
  { id: "codex", name: "Codex" },
  { id: "gemini", name: "Gemini CLI" },
  { id: "opencode", name: "OpenCode" },
  { id: "copilot", name: "GitHub Copilot CLI" },
  { id: "cursor", name: "Cursor Agent" },
  { id: "pi", name: "Pi" },
  { id: "kimi-cli", name: "Kimi CLI" },
  { id: "qwen-code", name: "Qwen Code" },
  { id: "openrouter", name: "OpenRouter" },
  { id: "zai", name: "z.ai" },
  { id: "moonshot", name: "Moonshot / Kimi" },
  { id: "vercel-ai-gateway", name: "Vercel AI Gateway" },
  { id: "xai", name: "xAI" },
] as const;

export type BuiltinProviderIntegration = (typeof BUILTIN_PROVIDER_INTEGRATIONS)[number];

export const BUILTIN_PROVIDER_COUNT = BUILTIN_PROVIDER_INTEGRATIONS.length;
