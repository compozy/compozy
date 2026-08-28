import type {
  RuntimeACPOption,
  RuntimeModelOption,
  RuntimeProviderOption,
} from "../runtime-selector";

// Truthful aggregate fixtures: two live providers plus a signed-out one, and a
// cross-provider model set carrying real curation / availability / reasoning
// metadata (a shared canonical id `gpt-5.6-sol` proves compound identity).
export const runtimeSelectorStoryProviders: RuntimeProviderOption[] = [
  { id: "codex", name: "Codex", runtime_provider: "codex", harness: "acp" },
  { id: "claude", name: "Claude", runtime_provider: "claude", harness: "acp" },
  { id: "openrouter", name: "OpenRouter", runtime_provider: "openrouter", harness: "pi_acp" },
  { id: "cursor", name: "Cursor", runtime_provider: "cursor", harness: "acp", needs_auth: true },
];

function runtimeModel(overrides: Partial<RuntimeModelOption> & { id: string; provider: string }) {
  return {
    name: overrides.id,
    efforts: [],
    availability: "live",
    curated: true,
    supports_tools: true,
    ...overrides,
  } satisfies RuntimeModelOption;
}

export const runtimeSelectorStoryModels: RuntimeModelOption[] = [
  runtimeModel({
    id: "gpt-5.6-sol",
    provider: "codex",
    name: "GPT-5.6 Sol",
    featured: true,
    context_window: 1_050_000,
    cost_input: 5,
    cost_output: 30,
    efforts: ["none", "low", "medium", "high", "xhigh", "max"],
    default_effort: "medium",
    reasoning_source: "acp",
    release_date: "2026-06-26",
  }),
  runtimeModel({
    id: "gpt-5.6-luna",
    provider: "codex",
    name: "GPT-5.6 Luna",
    context_window: 1_050_000,
    cost_input: 1,
    cost_output: 6,
    efforts: ["none", "low", "medium", "high", "xhigh", "max"],
    default_effort: "medium",
    reasoning_source: "acp",
  }),
  runtimeModel({
    id: "claude-fable-5",
    provider: "claude",
    name: "Claude Fable 5",
    featured: true,
    context_window: 1_000_000,
    cost_input: 10,
    cost_output: 50,
    efforts: ["low", "medium", "high", "xhigh", "max"],
    default_effort: "high",
    reasoning_source: "acp",
    release_date: "2026-06-09",
  }),
  runtimeModel({
    id: "claude-haiku-4-5-20251001",
    provider: "claude",
    name: "Claude Haiku 4.5",
    context_window: 200_000,
    cost_input: 1,
    cost_output: 5,
    // Canonical builtin metadata: supports_reasoning with no selectable effort
    // subset means the trigger drops its reasoning segment and the footer reads
    // "provider decides" (internal/config/provider_reasoning.go).
    supports_reasoning: true,
  }),
  runtimeModel({
    id: "gpt-5.4-mini",
    provider: "codex",
    name: "GPT-5.4 Mini",
    // Non-curated: hidden while browsing, revealed on search.
    curated: false,
    availability: "stale",
  }),
  // Same canonical id under a different provider is a distinct,
  // independently selectable and favoritable row.
  runtimeModel({
    id: "gpt-5.6-sol",
    provider: "openrouter",
    name: "GPT-5.6 Sol (via OpenRouter)",
    context_window: 1_050_000,
    cost_input: 6,
    cost_output: 32,
    efforts: ["low", "medium", "high"],
    reasoning_source: "catalog",
  }),
];

export const runtimeSelectorStoryAdvancedOptions: RuntimeACPOption[] = [
  {
    id: "context_window",
    label: "Context window",
    description: "Maximum context for the next run",
    kind: "select",
    current_value_id: "200k",
    values: [
      {
        value: "200k",
        label: "200k tokens",
        group_id: "standard",
        group_label: "Standard",
      },
      {
        value: "1m",
        label: "1M tokens",
        group_id: "large",
        group_label: "Large",
      },
    ],
  },
  {
    id: "thinking",
    label: "Thinking",
    description: "Allow the provider to spend more time reasoning",
    kind: "boolean",
    current_bool: false,
  },
];
