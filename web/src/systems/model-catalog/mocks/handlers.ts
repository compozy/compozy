import type { HttpHandler } from "msw";
import { HttpResponse } from "msw";

import { aghApiMock } from "@/storybook/openapi-msw";

import type { ProviderModelPayload } from "../types";

function model(
  overrides: Partial<ProviderModelPayload> & {
    provider_id: string;
    model_id: string;
  }
): ProviderModelPayload {
  return {
    availability_state: "available_live",
    available: true,
    curated: true,
    deprecated: false,
    featured: false,
    hidden: false,
    stale: false,
    sources: [
      {
        source_id: "builtin",
        source_kind: "builtin",
        priority: 1,
        stale: false,
        refreshed_at: "2026-04-17T18:00:00Z",
      },
    ],
    display_name: overrides.model_id,
    ...overrides,
  };
}

/** Cross-provider aggregate used by RuntimeSelector on agent detail/settings. */
export const modelCatalogAllModelsFixture: ProviderModelPayload[] = [
  model({
    provider_id: "claude",
    model_id: "claude-fable-5",
    display_name: "Claude Fable 5",
    featured: true,
    context_window: 1_000_000,
    cost: { input_per_million: 10, output_per_million: 50 },
    reasoning_efforts: ["low", "medium", "high", "xhigh", "max"],
    default_reasoning_effort: "high",
    reasoning_source: "acp",
    supports_tools: true,
  }),
  model({
    provider_id: "claude",
    model_id: "claude-haiku-4-5-20251001",
    display_name: "Claude Haiku 4.5",
    context_window: 200_000,
    cost: { input_per_million: 1, output_per_million: 5 },
    supports_reasoning: true,
    supports_tools: true,
  }),
  model({
    provider_id: "codex",
    model_id: "gpt-5.6-sol",
    display_name: "GPT-5.6 Sol",
    featured: true,
    context_window: 1_050_000,
    cost: { input_per_million: 5, output_per_million: 30 },
    reasoning_efforts: ["none", "low", "medium", "high", "xhigh", "max"],
    default_reasoning_effort: "medium",
    reasoning_source: "acp",
    supports_tools: true,
  }),
];

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/model-catalog/models", () =>
    HttpResponse.json({ models: modelCatalogAllModelsFixture })
  ),
  aghApiMock.post("/api/model-catalog/models/refresh", () =>
    HttpResponse.json({
      sources: [
        {
          provider_id: "claude",
          source_id: "builtin",
          source_kind: "builtin",
          priority: 1,
          refresh_state: "ready",
          row_count: modelCatalogAllModelsFixture.length,
          stale: false,
          last_refresh: "2026-04-17T18:00:00Z",
          last_success: "2026-04-17T18:00:00Z",
        },
      ],
    })
  ),
  aghApiMock.get("/api/model-catalog/providers/{provider_id}/models", ({ params }) => {
    const providerId = String(params.provider_id);
    return HttpResponse.json({
      models: modelCatalogAllModelsFixture.filter(row => row.provider_id === providerId),
    });
  }),
  aghApiMock.get("/api/model-catalog/providers/{provider_id}/models/status", ({ params }) =>
    HttpResponse.json({
      sources: [
        {
          provider_id: String(params.provider_id),
          source_id: "builtin",
          source_kind: "builtin",
          priority: 1,
          refresh_state: "ready",
          row_count: modelCatalogAllModelsFixture.filter(
            row => row.provider_id === String(params.provider_id)
          ).length,
          stale: false,
          last_refresh: "2026-04-17T18:00:00Z",
          last_success: "2026-04-17T18:00:00Z",
        },
      ],
    })
  ),
  aghApiMock.post("/api/model-catalog/providers/{provider_id}/models/refresh", ({ params }) =>
    HttpResponse.json({
      sources: [
        {
          provider_id: String(params.provider_id),
          source_id: "builtin",
          source_kind: "builtin",
          priority: 1,
          refresh_state: "ready",
          row_count: modelCatalogAllModelsFixture.filter(
            row => row.provider_id === String(params.provider_id)
          ).length,
          stale: false,
          last_refresh: "2026-04-17T18:00:00Z",
          last_success: "2026-04-17T18:00:00Z",
        },
      ],
    })
  ),
];
