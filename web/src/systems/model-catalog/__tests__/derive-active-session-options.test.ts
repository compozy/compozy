import { describe, expect, it } from "vitest";

import { deriveActiveSessionOptions } from "../lib/derive-active-session-options";
import type { ProviderModelPayload } from "../types";

const visibleCatalogFlags = {
  curated: true,
  deprecated: false,
  featured: false,
  hidden: false,
} satisfies Pick<ProviderModelPayload, "curated" | "deprecated" | "featured" | "hidden">;

const codexCatalog: ProviderModelPayload[] = [
  {
    ...visibleCatalogFlags,
    provider_id: "codex",
    model_id: "gpt-5.4",
    display_name: "GPT-5.4",
    availability_state: "available_live",
    available: true,
    stale: false,
    refreshed_at: "2026-05-07T10:00:00Z",
    sources: [
      {
        source_id: "config",
        source_kind: "config",
        priority: 120,
        refreshed_at: "2026-05-07T10:00:00Z",
        stale: false,
      },
    ],
    supports_reasoning: true,
    reasoning_efforts: ["low", "medium", "high"],
    default_reasoning_effort: "medium",
  },
  {
    ...visibleCatalogFlags,
    provider_id: "codex",
    model_id: "gpt-5.5",
    display_name: "GPT-5.5",
    availability_state: "available_live",
    available: true,
    stale: false,
    refreshed_at: "2026-05-07T10:00:00Z",
    sources: [
      {
        source_id: "models_dev",
        source_kind: "models_dev",
        priority: 50,
        refreshed_at: "2026-05-07T10:00:00Z",
        stale: false,
      },
    ],
    supports_reasoning: true,
    reasoning_efforts: ["minimal", "low", "medium", "high", "xhigh"],
    default_reasoning_effort: "medium",
  },
  {
    ...visibleCatalogFlags,
    provider_id: "codex",
    model_id: "gpt-5.4-mini",
    display_name: "GPT-5.4 Mini",
    availability_state: "available_stale",
    available: true,
    stale: true,
    refreshed_at: "2026-05-06T10:00:00Z",
    sources: [
      {
        source_id: "models_dev",
        source_kind: "models_dev",
        priority: 50,
        refreshed_at: "2026-05-06T10:00:00Z",
        stale: true,
      },
    ],
    supports_reasoning: false,
  },
];

describe("deriveActiveSessionOptions", () => {
  it("Should derive catalog model options when no ACP config option is present", () => {
    const result = deriveActiveSessionOptions({
      catalog: codexCatalog,
      selectedModel: "gpt-5.4",
    });

    expect(result.modelOptions).toHaveLength(3);
    expect(result.modelOptions[0]).toMatchObject({
      id: "gpt-5.4",
      availabilityState: "available_live",
      source: "catalog",
    });
    expect(result.modelOverrideAvailable).toBe(false);
  });

  it("Should prefer ACP model values when an ACP config option is provided", () => {
    const result = deriveActiveSessionOptions({
      catalog: codexCatalog,
      configOptions: [
        {
          id: "model",
          kind: "enum",
          current: "gpt-5.4",
          values: [
            { value: "gpt-5.4", label: "Active default" },
            { value: "experimental-routing", label: "Experimental" },
          ],
        },
      ],
      selectedModel: "gpt-5.4",
    });

    expect(result.modelOverrideAvailable).toBe(true);
    expect(result.modelOptions.map(option => option.id)).toEqual([
      "experimental-routing",
      "gpt-5.4",
    ]);
    const enriched = result.modelOptions.find(option => option.id === "gpt-5.4");
    expect(enriched?.availabilityState).toBe("available_live");
    expect(enriched?.source).toBe("acp");
    expect(enriched?.displayName).toBe("Active default");
    const experimental = result.modelOptions.find(option => option.id === "experimental-routing");
    expect(experimental?.availabilityState).toBe("unknown");
    expect(experimental?.available).toBeNull();
  });

  it("Should derive reasoning options from the selected catalog row when no ACP option exists", () => {
    const result = deriveActiveSessionOptions({
      catalog: codexCatalog,
      selectedModel: "gpt-5.4",
    });

    expect(result.reasoningSupported).toBe(true);
    expect(result.reasoningOptions.map(option => option.value)).toEqual(["low", "medium", "high"]);
    expect(result.reasoningOverrideAvailable).toBe(false);
    expect(result.defaultReasoning).toBe("medium");
    // No ACP config option and no adapter-advertised source → catalog authority.
    expect(result.reasoningSource).toBe("catalog");
    expect(result.reasoningOptions.every(option => option.source === "catalog")).toBe(true);
  });

  it("Should mark reasoningSource acp when the selected catalog row is adapter-advertised", () => {
    const result = deriveActiveSessionOptions({
      catalog: [
        {
          ...visibleCatalogFlags,
          provider_id: "claude",
          model_id: "claude-sonnet-5",
          availability_state: "available_live",
          available: true,
          stale: false,
          sources: [],
          supports_reasoning: true,
          reasoning_efforts: ["low", "medium", "high"],
          reasoning_source: "acp",
        },
      ],
      selectedModel: "claude-sonnet-5",
    });

    expect(result.reasoningSupported).toBe(true);
    expect(result.reasoningSource).toBe("acp");
    expect(result.reasoningOptions.every(option => option.source === "acp")).toBe(true);
  });

  it("Should derive reasoning options from an enriched catalog row", () => {
    const result = deriveActiveSessionOptions({
      catalog: codexCatalog,
      selectedModel: "gpt-5.5",
    });

    expect(result.reasoningSupported).toBe(true);
    expect(result.reasoningOptions.map(option => option.value)).toEqual([
      "minimal",
      "low",
      "medium",
      "high",
      "xhigh",
    ]);
    expect(result.defaultReasoning).toBe("medium");
  });

  it("Should disable reasoning when the selected catalog row does not support it", () => {
    const result = deriveActiveSessionOptions({
      catalog: codexCatalog,
      selectedModel: "gpt-5.4-mini",
    });

    expect(result.reasoningSupported).toBe(false);
    expect(result.reasoningOptions).toEqual([]);
  });

  it("Should disable reasoning when the selected catalog row has no support signal or efforts", () => {
    const result = deriveActiveSessionOptions({
      catalog: [
        {
          ...visibleCatalogFlags,
          provider_id: "custom",
          model_id: "custom-chat-model",
          availability_state: "unknown",
          available: null,
          stale: false,
          sources: [],
        },
      ],
      selectedModel: "custom-chat-model",
    });

    expect(result.reasoningSupported).toBe(false);
    expect(result.reasoningOptions).toEqual([]);
    expect(result.defaultReasoning).toBeNull();
  });

  it("Should override reasoning options with ACP config values when present", () => {
    const result = deriveActiveSessionOptions({
      catalog: codexCatalog,
      selectedModel: "gpt-5.4-mini",
      configOptions: [
        {
          id: "reasoning_effort",
          kind: "enum",
          current: "high",
          values: [
            { value: "low", label: "Low" },
            { value: "high", label: "High" },
          ],
        },
      ],
    });

    expect(result.reasoningSupported).toBe(true);
    expect(result.reasoningOverrideAvailable).toBe(true);
    expect(result.reasoningOptions).toEqual([
      { value: "low", label: "Low", source: "acp" },
      { value: "high", label: "High", source: "acp" },
    ]);
    expect(result.defaultReasoning).toBe("high");
    // A live ACP config option is authoritative for the source.
    expect(result.reasoningSource).toBe("acp");
  });

  it("Should treat catalog rows as the only authority when ACP exposes no values for reasoning", () => {
    const result = deriveActiveSessionOptions({
      catalog: codexCatalog,
      selectedModel: "gpt-5.4",
      configOptions: [
        {
          id: "reasoning_effort",
          kind: "enum",
          values: [],
        },
      ],
    });

    expect(result.reasoningOverrideAvailable).toBe(true);
    expect(result.reasoningOptions).toEqual([]);
  });
});
