import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import type { VaultSecret } from "@/systems/vault/types";

import { SessionVaultPanel } from "../session-vault-panel";

const secrets: VaultSecret[] = [
  {
    ref: "vault:sessions/session_launch_coordination/stripe_api_key",
    namespace: "sessions",
    kind: "api_key",
    present: true,
    created_at: "2026-04-17T18:00:00Z",
    updated_at: "2026-04-17T18:14:00Z",
  },
  {
    ref: "vault:sessions/session_launch_coordination/partner_webhook_secret",
    namespace: "sessions",
    kind: "webhook",
    present: true,
    created_at: "2026-04-17T18:02:00Z",
    updated_at: "2026-04-17T18:18:00Z",
  },
];

const denseSecrets = Array.from(
  { length: 8 },
  (_, index): VaultSecret => ({
    ref: `vault:sessions/session_launch_coordination/generated_secret_${index + 1}`,
    namespace: "sessions",
    kind: index % 2 === 0 ? "api_key" : "webhook",
    present: true,
    created_at: "2026-04-17T18:00:00Z",
    updated_at: "2026-04-17T18:14:00Z",
  })
);

const meta: Meta<typeof SessionVaultPanel> = {
  title: "systems/vault/components/SessionVaultPanel",
  component: SessionVaultPanel,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Compact session-scoped vault metadata panel used inside the session inspector.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-full max-w-lg rounded-lg border border-line bg-canvas-soft p-5">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Session refs are shortened when the current session id is known.
 */
export const Default: Story = {
  args: {
    secrets,
    sessionId: "session_launch_coordination",
  },
};

/**
 * Loading state keeps the inspector panel centered.
 */
export const Loading: Story = {
  args: {
    secrets: [],
    isLoading: true,
  },
};

/**
 * Empty copy explains that secret values remain write-only.
 */
export const Empty: Story = {
  args: {
    secrets: [],
    sessionId: "session_launch_coordination",
  },
};

/**
 * Error state renders daemon-facing failure details.
 */
export const ErrorState: Story = {
  args: {
    secrets: [],
    error: new Error("Vault metadata endpoint returned 503."),
  },
};

/**
 * Dense state exercises Item rows and truncation inside the inspector panel.
 */
export const Dense: Story = {
  args: {
    secrets: denseSecrets,
    sessionId: "session_launch_coordination",
  },
};
