import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { useState } from "react";
import { fn } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";
import type { ProviderDraft } from "@/systems/settings";
import {
  emptyProviderDraft,
  providerDraftFromEntry,
  withProviderAuthMode,
} from "@/systems/settings/lib/provider-draft";
import { settingsProviderFixtures } from "@/systems/settings/mocks";

import { ProviderDetailDialog } from "../provider-detail-dialog";
import type { ProviderInspectorState } from "@/systems/settings/hooks/use-settings-providers-page";

const claude = settingsProviderFixtures[0]!;
const openrouter = settingsProviderFixtures.find(entry => entry.name === "openrouter")!;

/**
 * Vault-bound twin of the OpenRouter fixture: only a `vault:` ref can hold a
 * value AGH wrote, so rotation is reachable only from this shape.
 */
const openrouterVaultBound = {
  ...openrouter,
  settings: {
    ...openrouter.settings,
    credential_slots: [
      {
        name: "api_key",
        target_env: "OPENROUTER_API_KEY",
        secret_ref: "vault:providers/openrouter/api-key",
        kind: "api_key",
        required: true,
      },
    ],
  },
  credentials: [
    {
      name: "api_key",
      target_env: "OPENROUTER_API_KEY",
      secret_ref: "vault:providers/openrouter/api-key",
      kind: "api_key",
      required: true,
      present: true,
    },
  ],
};

const freshHandlers = [
  aghApiMock.get("/api/model-catalog/providers/{provider_id}/models/status", () =>
    HttpResponse.json({
      sources: [
        {
          source_id: "models.dev",
          source_kind: "models_dev",
          priority: 0,
          provider_id: "claude",
          refresh_state: "succeeded",
          row_count: 42,
          stale: false,
          last_success: "2026-04-17T18:10:00Z",
        },
      ],
    })
  ),
  aghApiMock.post("/api/model-catalog/providers/{provider_id}/models/refresh", () =>
    HttpResponse.json({
      sources: [
        {
          source_id: "models.dev",
          source_kind: "models_dev",
          priority: 0,
          provider_id: "claude",
          refresh_state: "queued",
          row_count: 42,
          stale: false,
          next_refresh: "2026-04-17T18:12:00Z",
        },
      ],
    })
  ),
];

interface HarnessProps {
  initialState: ProviderInspectorState;
}

function createDraft(overrides: Partial<ProviderDraft> = {}): ProviderDraft {
  return { ...emptyProviderDraft(), ...overrides };
}

function boundSecretCreateDraft(): ProviderDraft {
  const seeded = withProviderAuthMode(
    createDraft({ name: "openai", display_name: "OpenAI", command: "codex" }),
    "bound_secret"
  );
  return {
    ...seeded,
    credential_slots: [
      {
        key: "credential-slot-story",
        name: "api_key",
        target_env: "OPENAI_API_KEY",
        secret_ref: "vault:providers/openai/api-key",
        kind: "api_key",
        required: true,
      },
    ],
    credential_secret_values: [""],
  };
}

function Harness({ initialState }: HarnessProps) {
  const [state, setState] = useState<ProviderInspectorState>(initialState);
  const entry = state.mode === "inspect" || state.mode === "edit" ? state.entry : null;
  const draft = state.mode === "edit" || state.mode === "create" ? state.draft : null;

  return (
    <StorySurface className="min-h-160 bg-canvas p-6">
      <ProviderDetailDialog
        canSave={true}
        draft={draft}
        entry={entry}
        error={null}
        isDeleting={false}
        isSaving={false}
        mode={state.mode === "closed" ? "inspect" : state.mode}
        onCancelEdit={() => {
          setState(current => {
            if (current.mode === "edit" && current.cameFrom === "inspect") {
              return { mode: "inspect", entry: current.entry };
            }
            return { mode: "closed" };
          });
        }}
        onDraftChange={updater => {
          setState(current => {
            if (current.mode !== "edit" && current.mode !== "create") return current;
            return { ...current, draft: updater(current.draft) };
          });
        }}
        onOpenChange={next => {
          if (!next) setState({ mode: "closed" });
        }}
        onRefreshCatalog={fn()}
        onRequestDelete={fn()}
        onSave={fn()}
        onSwitchToEdit={() => {
          setState(current => {
            if (current.mode !== "inspect") return current;
            return {
              mode: "edit",
              entry: current.entry,
              draft: providerDraftFromEntry(current.entry),
              cameFrom: "inspect",
            };
          });
        }}
        open={state.mode !== "closed"}
        warnings={undefined}
      />
    </StorySurface>
  );
}

const meta: Meta<typeof ProviderDetailDialog> = {
  title: "systems/settings/components/ProviderDetailDialog",
  component: ProviderDetailDialog,
  parameters: {
    layout: "fullscreen",
    msw: { handlers: freshHandlers },
    docs: {
      description: {
        component:
          "Provider inspect, edit, and create on one dialog host (D1(b)). The body follows the designed provider-sheet grammar: SettingsFieldRow rows, auth-ownership RadioCards, and credential slots that exist only under bound_secret.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const InspectClaudeDefault: Story = {
  args: {},
  render: () => <Harness initialState={{ mode: "inspect", entry: claude }} />,
};

export const InspectUnconfigured: Story = {
  args: {},
  render: () => <Harness initialState={{ mode: "inspect", entry: openrouter }} />,
};

export const EditMode: Story = {
  args: {},
  render: () => (
    <Harness
      initialState={{
        mode: "edit",
        entry: claude,
        draft: providerDraftFromEntry(claude),
        cameFrom: "inspect",
      }}
    />
  ),
};

/** Rotation-only credential state: presence is reported, the value is never read back. */
export const EditBoundSecretRotate: Story = {
  args: {},
  render: () => (
    <Harness
      initialState={{
        mode: "edit",
        entry: openrouterVaultBound,
        draft: providerDraftFromEntry(openrouterVaultBound),
        cameFrom: "inspect",
      }}
    />
  ),
};

export const CreateNativeCli: Story = {
  args: {},
  render: () => (
    <Harness
      initialState={{
        mode: "create",
        draft: createDraft({
          name: "codex",
          display_name: "Codex",
          command: "codex",
          auth_login_command: "codex login",
          auth_status_command: "codex auth status",
        }),
      }}
    />
  ),
};

/** The auth gate: credential slots exist only under `bound_secret`. */
export const CreateBoundSecret: Story = {
  args: {},
  render: () => <Harness initialState={{ mode: "create", draft: boundSecretCreateDraft() }} />,
};
