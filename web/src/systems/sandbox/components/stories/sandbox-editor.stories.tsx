import type { Meta, StoryObj } from "@storybook/react-vite";

import type { EntityMode } from "@agh/ui";

import type { SettingsSandboxEntry } from "@/systems/settings";

import { SandboxEditor } from "../sandbox-editor";
import { emptySandboxDraft, type SandboxDraft } from "../../lib/sandbox-profile-draft";

const entry: SettingsSandboxEntry = {
  name: "isolated-dev",
  profile: { backend: "daytona" },
  source_metadata: {
    available_targets: ["global-config"],
    effective_source: { kind: "global-config", scope: "global" },
  },
  workspace_usage_count: 2,
};

const localDraft: SandboxDraft = {
  ...emptySandboxDraft(),
  name: "isolated-dev",
  backend: "local",
  sync_mode: "session-bidirectional",
  persistence: "transient",
  runtime_root: "/workspace",
  env: [{ key: "LOG_LEVEL", value: "info" }],
  secretEnv: [{ key: "GITHUB_TOKEN", value: "vault:sandbox/github/token" }],
  network: { allow_outbound: true, allow_list: ["api.github.com"] },
};

const daytonaDraft: SandboxDraft = {
  ...localDraft,
  backend: "daytona",
  network: { allow_public_ingress: true },
  daytona: { api_url: "https://app.daytona.io/api", class: "medium", auto_stop: "30m" },
};

const meta: Meta<typeof SandboxEditor> = {
  title: "systems/sandbox/components/SandboxEditor",
  component: SandboxEditor,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Sandbox profile editor. Simple names the profile and picks the execution backend; Advanced carries lifecycle, environment, network policy, and — only for Daytona — the cloud workspace parameters. `secret_env` holds vault/env references, never plaintext.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function harness(
  overrides: Partial<Parameters<typeof SandboxEditor>[0]> & {
    draft?: SandboxDraft;
    tier?: EntityMode;
    mode?: "create" | "edit";
  } = {}
) {
  const { draft = localDraft, tier = "simple", mode = "edit", ...rest } = overrides;
  return (
    <SandboxEditor
      editor={
        mode === "create"
          ? { mode: "create", draft }
          : { mode: "edit", name: draft.name, draft, entry }
      }
      error={null}
      existingNames={["local"]}
      initialTier={tier}
      isSaving={false}
      isValid
      onChange={() => undefined}
      onClose={() => undefined}
      onSave={() => undefined}
      {...rest}
    />
  );
}

/** Simple tier while creating: name, backend, and workspace sync. */
export const CreateSimple: Story = {
  args: {},
  render: () => harness({ draft: { ...emptySandboxDraft(), name: "" }, mode: "create" }),
};

/** Simple tier on an existing profile — identity is locked. */
export const EditSimple: Story = {
  args: {},
  render: () => harness(),
};

/** Advanced on a local profile: no Daytona block is rendered. */
export const AdvancedLocal: Story = {
  args: {},
  render: () => harness({ tier: "advanced" }),
};

/** Advanced on a Daytona profile: the cloud workspace block appears. */
export const AdvancedDaytona: Story = {
  args: {},
  render: () => harness({ draft: daytonaDraft, tier: "advanced" }),
};

/** A literal secret_env value blocks the save and names the reason. */
export const SecretEnvLiteralRejected: Story = {
  args: {},
  render: () =>
    harness({
      draft: { ...localDraft, secretEnv: [{ key: "GITHUB_TOKEN", value: "ghp_liveTokenValue" }] },
      tier: "advanced",
    }),
};

/** A backend this editor cannot configure is reported, never silently rewritten. */
export const UnlistedBackend: Story = {
  args: {},
  render: () => harness({ draft: { ...localDraft, backend: "e2b" } }),
};

/** Save failure keeps every entered value in place. */
export const SaveError: Story = {
  args: {},
  render: () =>
    harness({ error: "The daemon rejected the replace: sandbox profile is referenced." }),
};
