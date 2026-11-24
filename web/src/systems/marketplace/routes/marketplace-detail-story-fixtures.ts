import { HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import { extensionFixtures } from "@/systems/extensions";

export const kitExtensionFixture = {
  ...extensionFixtures[0]!,
  bound_env_keys: ["DEP_KIT_TOKEN"],
  enabled: false,
  missing_env: ["DEP_KIT_WEBHOOK"],
  name: "dep-kit-ops",
  inputs: [],
  declared_profiles: [
    {
      created_by_extension: true,
      credential_requirements: [
        {
          missing: true,
          provider: "openai",
          slot: "api_key",
          source_extension: "dep-kit-ops",
        },
      ],
      exists: true,
      name: "growth",
      needs_setup: true,
    },
    {
      created_by_extension: true,
      credential_requirements: [],
      exists: true,
      name: "operations",
      needs_setup: false,
    },
  ],
  placements: [
    { dormant: false, kind: "skill", profile: "growth", resource: "campaign-brief" },
    { dormant: false, kind: "agent", resource: "release-reviewer" },
  ],
  dormant_placements: [
    {
      create_action: "Create studio profile",
      dormant: true,
      kind: "layout",
      profile: "studio",
      resource: "campaign-board",
    },
  ],
  remote_version: undefined,
  requires_env: ["DEP_KIT_TOKEN", "DEP_KIT_WEBHOOK"],
  update_available: false,
  version: "1.0.0",
};

export const kitInventoryItems = [
  { id: "agent:dep-reviewer", kind: "agent", live: false, name: "dep-reviewer" },
  { id: "agent:release-notes", kind: "agent", live: true, name: "release-notes" },
  { id: "automation:weekly-audit", kind: "automation", live: false, name: "weekly-audit" },
  { id: "layout:dep-board", kind: "layout", live: false, name: "dep-board" },
];

/** One MSW group set per story: a second `storybookMswParameters` spread would replace the first. */
export function kitDetailHandlers() {
  return storybookMswParameters({
    marketplace: [
      compozyApiMock.get("/api/marketplace/entries/{entry_id}", () =>
        HttpResponse.json({
          entry: {
            category: "development",
            category_label: "Development",
            featured: false,
            description: "Dependency review agents, a weekly sweep, and a review board layout.",
            digest_sha256: "a".repeat(64),
            entry_id: "dep-kit-ops",
            install_slug: "compozy/dep-kit-ops",
            installable: true,
            installed: true,
            installed_name: "dep-kit-ops",
            installed_version: "1.0.0",
            manage_path: "/marketplace/installed",
            name: "dep-kit-ops",
            update_available: false,
            source: "compozy-catalog",
            source_ref: "catalog:compozy",
          },
        })
      ),
    ],
    extensions: [
      compozyApiMock.get("/api/extensions", () =>
        HttpResponse.json({
          extensions: [
            {
              ...kitExtensionFixture,
            },
          ],
        })
      ),
      compozyApiMock.get("/api/extensions/{name}/inventory", () =>
        HttpResponse.json({
          enabled: false,
          extension: "dep-kit-ops",
          format: "compozy",
          items: kitInventoryItems,
        })
      ),
    ],
  });
}
