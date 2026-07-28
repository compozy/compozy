import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { PanelSurface } from "@/storybook/story-layout";
import type { VaultSecret } from "@/systems/vault/types";

import { VaultSecretsList } from "../vault-secrets-list";

const secrets: VaultSecret[] = [
  {
    ref: "vault:providers/codex/api_key",
    namespace: "providers",
    kind: "api_key",
    present: true,
    created_at: "2026-04-17T17:30:00Z",
    updated_at: "2026-04-17T17:42:00Z",
  },
  {
    ref: "vault:bridges/slack/signing_secret",
    namespace: "bridges",
    kind: "signing_secret",
    present: true,
    created_at: "2026-04-17T17:31:00Z",
    updated_at: "2026-04-17T17:55:00Z",
  },
  {
    ref: "vault:sessions/session_launch_coordination/partner_webhook_secret",
    namespace: "sessions",
    kind: "webhook",
    present: true,
    created_at: "2026-04-17T17:58:00Z",
    updated_at: "2026-04-17T18:18:00Z",
  },
];

const denseSecrets = Array.from(
  { length: 12 },
  (_, index): VaultSecret => ({
    ref: `vault:providers/provider_${index + 1}/api_key_with_a_very_long_metadata_reference_${index + 1}`,
    namespace: index % 3 === 0 ? "sessions" : "providers",
    kind: index % 2 === 0 ? "api_key" : "webhook",
    present: true,
    created_at: "2026-04-17T17:30:00Z",
    updated_at: "2026-04-17T17:42:00Z",
  })
);

const meta: Meta<typeof VaultSecretsList> = {
  title: "systems/vault/components/VaultSecretsList",
  component: VaultSecretsList,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Vault metadata listing rows (mono refs + namespace Pills) inside the shared bordered list card, with optional destructive actions.",
      },
    },
  },
  decorators: [
    Story => (
      <PanelSurface className="min-h-[420px] p-6">
        <Story />
      </PanelSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Interactive rows shown on the vault page. */
export const Default: Story = {
  args: {
    secrets,
    onSelect: fn(),
    onDelete: fn(),
  },
};

/** Cards view mirrors the OpenDesign listing toggle. */
export const Cards: Story = {
  args: {
    secrets,
    view: "cards",
    onSelect: fn(),
    onDelete: fn(),
  },
};

/** Delete actions appear only when the parent route wires the mutation. */
export const WithDeleteActions: Story = {
  args: {
    secrets,
    onSelect: fn(),
    onDelete: fn(),
  },
};

/** Loading state preserves the list footprint while metadata resolves. */
export const Loading: Story = {
  args: {
    secrets: [],
    isLoading: true,
  },
};

/** Empty state uses caller-provided copy for scoped vault views. */
export const Empty: Story = {
  args: {
    secrets: [],
    emptyTitle: "No provider secrets",
    emptyDescription: "Provider credential metadata appears here after a secret is stored.",
  },
};

/** Error state keeps daemon failure copy visible in the shared data surface. */
export const Error: Story = {
  args: {
    secrets: [],
    error: new globalThis.Error("Vault metadata endpoint returned 503."),
  },
};

/** Dense state exercises long refs and mixed namespaces. */
export const Dense: Story = {
  args: {
    secrets: denseSecrets,
    onDelete: fn(),
  },
};
