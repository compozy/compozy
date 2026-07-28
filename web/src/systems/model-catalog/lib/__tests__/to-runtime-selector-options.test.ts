import { describe, expect, it } from "vitest";

import type { ProviderModelPayload } from "../../types";
import { providerNeedsAuth, toRuntimeModelOptions } from "../to-runtime-selector-options";

function payload(
  overrides: Partial<ProviderModelPayload> & { model_id: string }
): ProviderModelPayload {
  return {
    provider_id: "codex",
    availability_state: "available_live",
    available: true,
    curated: true,
    deprecated: false,
    featured: false,
    hidden: false,
    stale: false,
    sources: [],
    ...overrides,
  };
}

describe("toRuntimeModelOptions", () => {
  it("Should map availability_state onto the live/stale/unavailable enum", () => {
    const result = toRuntimeModelOptions([
      payload({ model_id: "live", availability_state: "available_live" }),
      payload({ model_id: "stale", availability_state: "available_stale", stale: true }),
      payload({ model_id: "down-live", availability_state: "unavailable_live", available: false }),
      payload({
        model_id: "down-stale",
        availability_state: "unavailable_stale",
        available: false,
      }),
    ]);

    expect(result.map(option => [option.id, option.availability])).toEqual([
      ["live", "live"],
      ["stale", "stale"],
      ["down-live", "unavailable"],
      ["down-stale", "unavailable"],
    ]);
  });

  it("Should fall back to the available flag for unknown availability states", () => {
    const result = toRuntimeModelOptions([
      payload({ model_id: "unknown-false", availability_state: "unknown", available: false }),
      payload({ model_id: "unknown-true", availability_state: "unknown", available: true }),
      payload({ model_id: "unknown-null", availability_state: "unknown", available: null }),
    ]);

    expect(result.find(option => option.id === "unknown-false")?.availability).toBe("unavailable");
    expect(result.find(option => option.id === "unknown-true")?.availability).toBe("live");
    expect(result.find(option => option.id === "unknown-null")?.availability).toBe("live");
  });

  it("Should mark unavailable models as disabled with an Unavailable reason", () => {
    const [live, down] = toRuntimeModelOptions([
      payload({ model_id: "live", availability_state: "available_live" }),
      payload({ model_id: "down", availability_state: "unavailable_live", available: false }),
    ]);

    expect(live.disabled).toBe(false);
    expect(live.disabled_reason).toBeUndefined();
    expect(down.disabled).toBe(true);
    expect(down.disabled_reason).toBe("Unavailable");
  });

  it("Should disable every row with a Sign in reason when the provider needs auth", () => {
    const result = toRuntimeModelOptions(
      [
        payload({ model_id: "live", availability_state: "available_live" }),
        payload({ model_id: "stale", availability_state: "available_stale", stale: true }),
      ],
      { providerNeedsAuth: true }
    );

    expect(result.every(option => option.disabled)).toBe(true);
    expect(result.every(option => option.availability === "unavailable")).toBe(true);
    expect(result.every(option => option.disabled_reason === "Sign in")).toBe(true);
  });

  it("Should filter reasoning efforts down to canonical enum members", () => {
    const [option] = toRuntimeModelOptions([
      payload({
        model_id: "gpt",
        reasoning_efforts: [
          "low",
          "bogus",
          "high",
          "ultra",
        ] as ProviderModelPayload["reasoning_efforts"],
        default_reasoning_effort: "high",
        reasoning_source: "acp",
      }),
    ]);

    expect(option.efforts).toEqual(["low", "high"]);
    expect(option.default_effort).toBe("high");
    expect(option.reasoning_source).toBe("acp");
  });

  it("Should drop an off-contract default reasoning effort to provider default", () => {
    const [option] = toRuntimeModelOptions([
      payload({
        model_id: "gpt",
        reasoning_efforts: ["low", "high"],
        // "ultra" is not a canonical effort — it must never survive as a default.
        default_reasoning_effort: "ultra" as ProviderModelPayload["default_reasoning_effort"],
      }),
    ]);

    expect(option.efforts).toEqual(["low", "high"]);
    expect(option.default_effort).toBe("");
  });

  it("Should drop a canonical default that is outside the model's effort subset", () => {
    const [option] = toRuntimeModelOptions([
      payload({
        model_id: "gpt",
        reasoning_efforts: ["low", "high"],
        // "max" is canonical but this model can only honor low/high — clear it.
        default_reasoning_effort: "max",
      }),
    ]);

    expect(option.efforts).toEqual(["low", "high"]);
    expect(option.default_effort).toBe("");
  });

  it("Should pass through cost, context window, and capability flags", () => {
    const [option] = toRuntimeModelOptions([
      payload({
        model_id: "gpt",
        display_name: "  GPT Five  ",
        context_window: 200000,
        cost: {
          input_per_million: 3,
          output_per_million: 12,
          cache_read_per_million: 0.3,
          cache_write_per_million: 4,
          reasoning_per_million: 15,
        },
        supports_tools: true,
        supports_reasoning: true,
        featured: true,
      }),
    ]);

    expect(option.name).toBe("GPT Five");
    expect(option.context_window).toBe(200000);
    expect(option.cost_input).toBe(3);
    expect(option.cost_output).toBe(12);
    expect(option.cost_cache_read).toBe(0.3);
    expect(option.cost_cache_write).toBe(4);
    expect(option.cost_reasoning).toBe(15);
    expect(option.supports_tools).toBe(true);
    expect(option.supports_reasoning).toBe(true);
    expect(option.featured).toBe(true);
  });

  it("Should skip blank and same-provider duplicate model ids", () => {
    const result = toRuntimeModelOptions([
      payload({ model_id: "gpt" }),
      payload({ model_id: "  " }),
      payload({ model_id: "gpt", display_name: "Duplicate" }),
    ]);

    expect(result.map(option => option.id)).toEqual(["gpt"]);
    expect(result[0]?.name).toBe("gpt");
  });

  it("Should keep a shared model id for distinct providers (compound identity)", () => {
    const result = toRuntimeModelOptions([
      payload({ model_id: "shared", provider_id: "codex", display_name: "Shared on Codex" }),
      payload({ model_id: "shared", provider_id: "claude", display_name: "Shared on Claude" }),
    ]);

    expect(result).toHaveLength(2);
    expect(result.map(option => [option.provider, option.id])).toEqual([
      ["codex", "shared"],
      ["claude", "shared"],
    ]);
  });
});

describe("providerNeedsAuth", () => {
  it("Should treat needs_login and missing_credential as needing auth", () => {
    expect(providerNeedsAuth("needs_login")).toBe(true);
    expect(providerNeedsAuth("missing_credential")).toBe(true);
    expect(providerNeedsAuth("  needs_login  ")).toBe(true);
  });

  it("Should treat ready, unknown, and empty states as not needing auth", () => {
    expect(providerNeedsAuth("ready")).toBe(false);
    expect(providerNeedsAuth("")).toBe(false);
    expect(providerNeedsAuth(undefined)).toBe(false);
    expect(providerNeedsAuth(null)).toBe(false);
  });
});
