import { HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import type { ProviderModelPayload } from "@/systems/model-catalog";
import type { ProviderSummary } from "@/systems/providers";

function catalogProvider(name: string, displayName: string): ProviderSummary {
  return {
    name,
    display_name: displayName,
    default: false,
    auth_status: {
      mode: "native_cli",
      env_policy: "inherit",
      home_policy: "operator",
      state: "authenticated",
      login: { configured: true, presence: "present" },
    },
  };
}

function catalogModel(provider: string, id: string, displayName: string): ProviderModelPayload {
  return {
    provider_id: provider,
    model_id: id,
    display_name: displayName,
    availability_state: "available_live",
    available: true,
    curated: true,
    deprecated: false,
    featured: false,
    hidden: false,
    stale: false,
    sources: [],
  };
}

/** The runtime catalog the rows read their provider and model names from. */
export const MIXED_RUNTIME_CATALOG = [
  compozyApiMock.get("/api/providers", () =>
    HttpResponse.json({
      providers: [
        catalogProvider("claude", "Claude Code"),
        catalogProvider("codex", "Codex"),
        catalogProvider("gemini", "Gemini CLI"),
        catalogProvider("cursor", "Cursor Agent"),
      ],
    })
  ),
  compozyApiMock.get("/api/model-catalog/models", () =>
    HttpResponse.json({
      models: [
        catalogModel("claude", "claude-opus-5-5", "Claude Opus 5.5"),
        catalogModel("claude", "claude-sonnet-5-5", "Claude Sonnet 5.5"),
        catalogModel("codex", "gpt-6.1-sol", "GPT-6.1 Sol"),
        catalogModel("gemini", "gemini-3.1-pro", "Gemini 3.1 Pro"),
      ],
    })
  ),
];
