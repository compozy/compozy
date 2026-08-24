import { HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import { extensionFixtures } from "@/systems/extensions/mocks";

export const kitExtensionFixture = {
  ...extensionFixtures[0]!,
  bound_env_keys: ["DEP_KIT_TOKEN"],
  enabled: false,
  missing_env: ["DEP_KIT_WEBHOOK"],
  name: "dep-kit-ops",
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
  network_confirmation_required: true,
  network_requirement_digest: "sha256:6f1c0a94d3b27e58",
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
export function kitDetailHandlers(refuseUpdate = false) {
  return storybookMswParameters({
    marketplace: [
      compozyApiMock.get("/api/marketplace/{kind}/{entry_id}", () =>
        HttpResponse.json({
          entry: {
            description: "Dependency review agents, a weekly sweep, and a review board layout.",
            entry_id: "dep-kit-ops",
            installed: true,
            installed_name: "dep-kit-ops",
            installed_version: "1.0.0",
            kind: "extension",
            name: "dep-kit-ops",
            source: "registry",
            update_available: refuseUpdate,
            ...(refuseUpdate ? { version: "1.1.0" } : {}),
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
              ...(refuseUpdate ? { remote_version: "1.1.0", update_available: true } : {}),
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
      ...(refuseUpdate
        ? [
            compozyApiMock.put("/api/extensions/{name}", ({ response }) =>
              response(409).json({
                code: "extension_network_confirmation_required",
                current_digest: "sha256:6f1c0a94d3b27e58",
                error:
                  "dep-kit-ops update changes Live network participation that has not been confirmed",
              })
            ),
          ]
        : []),
    ],
  });
}
