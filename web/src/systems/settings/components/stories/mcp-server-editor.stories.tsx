import type { Meta, StoryObj } from "@storybook/react-vite";

import type { EntityMode } from "@agh/ui";

import { emptyDraft, type MCPDraft } from "../../lib/mcp-editor-model";
import { MCPServerEditor } from "../mcp-server-editor";

const localDraft: MCPDraft = {
  ...emptyDraft("stdio"),
  name: "github",
  command: "bunx @modelcontextprotocol/server-github",
  args: ["--read-only"],
  env: [{ key: "GITHUB_API_URL", value: "https://api.github.com" }],
};

const remoteDraft: MCPDraft = {
  ...emptyDraft("http"),
  name: "linear",
  url: "https://mcp.linear.app/mcp",
};

const configuredSecretDraft: MCPDraft = {
  ...localDraft,
  secretEnv: [
    {
      key: "GITHUB_TOKEN",
      binding: { mode: "preserve", existing: true, typedValue: "", vaultRef: "" },
    },
  ],
};

const meta: Meta<typeof MCPServerEditor> = {
  title: "systems/settings/components/MCPServerEditor",
  component: MCPServerEditor,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "MCP server editor. Simple carries the connection (local process or remote endpoint); Advanced carries the process environment for stdio or the OAuth block for a remote endpoint. Secrets are write-only and untouched bindings are preserved on save.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function harness(
  overrides: Partial<Parameters<typeof MCPServerEditor>[0]> & {
    draft?: MCPDraft;
    tier?: EntityMode;
  } = {}
) {
  const { draft = localDraft, tier = "simple", ...rest } = overrides;
  return (
    <MCPServerEditor
      availableTargets={["config"]}
      draft={draft}
      entry={null}
      errors={{}}
      initialTier={tier}
      isSaving={false}
      isValid
      mode="create"
      onChange={() => undefined}
      onClose={() => undefined}
      onSave={() => undefined}
      onTargetChange={() => undefined}
      open
      saveError={null}
      scope="workspace"
      target="config"
      vaultInventory={{ status: "ready", refs: ["vault:mcp/ws/ws-alpha/shared/GITHUB_TOKEN"] }}
      {...rest}
    />
  );
}

/** Simple tier while adding a local process server. */
export const CreateSimpleLocal: Story = {
  args: {},
  render: () => harness(),
};

/** Simple tier for a remote endpoint, with its wire transport exposed. */
export const CreateSimpleRemote: Story = {
  args: {},
  render: () => harness({ draft: remoteDraft }),
};

/** Advanced on a local server: args, env, and secret bindings. */
export const AdvancedLocal: Story = {
  args: {},
  render: () => harness({ tier: "advanced" }),
};

/** Advanced on a remote server: the OAuth block replaces the process block. */
export const EditAdvancedRemote: Story = {
  args: {},
  render: () => harness({ draft: remoteDraft, mode: "edit", tier: "advanced" }),
};

/** Editing locks the name and scope as readable identity. */
export const EditIdentityLocked: Story = {
  args: {},
  render: () => harness({ mode: "edit" }),
};

/** A configured secret offers preservation without disclosing the value. */
export const SecretPreserved: Story = {
  args: {},
  render: () => harness({ draft: configuredSecretDraft, mode: "edit", tier: "advanced" }),
};

/** A Vault-bound secret shows only the reference. */
export const SecretVaultRef: Story = {
  args: {},
  render: () =>
    harness({
      draft: {
        ...localDraft,
        secretEnv: [
          {
            key: "GITHUB_TOKEN",
            binding: {
              mode: "ref",
              existing: false,
              typedValue: "",
              vaultRef: "vault:mcp/ws/ws-alpha/shared/GITHUB_TOKEN",
            },
          },
        ],
      },
      tier: "advanced",
    }),
};

/** A rejected save keeps the draft in place. */
export const SaveError: Story = {
  args: {},
  render: () =>
    harness({ isValid: false, saveError: "The settings write rejected the MCP definition." }),
};
