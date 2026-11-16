import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fireEvent, userEvent, within } from "storybook/test";

import type { EntityMode } from "@compozy/ui";

import { emptyDraft, validateDraft, type MCPDraft } from "../../lib/mcp-editor-model";
import { MCPServerEditor } from "../mcp-server-editor";

const localDraft: MCPDraft = {
  ...emptyDraft("stdio"),
  name: "github",
  command: "npx",
  args: ["-y", "@modelcontextprotocol/server-github"],
  env: [{ key: "GITHUB_API_URL", value: "https://api.github.com" }],
  secretEnv: [
    {
      key: "GITHUB_TOKEN",
      binding: { mode: "typed", existing: false, typedValue: "", vaultRef: "" },
    },
  ],
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
      originalKey: "GITHUB_TOKEN",
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
          "Add or edit an MCP server: name and where to use it, how it connects (one URL-or-command field, or pasted mcpServers JSON, with a preview), the keys it needs, and a framed More options fold for arguments, settings, key names, OAuth and where it's saved. Secrets are write-only and untouched bindings are kept on save.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

type HarnessProps = Partial<Parameters<typeof MCPServerEditor>[0]> & {
  draft?: MCPDraft;
  tier?: EntityMode;
};

/** Stateful host: typing, picking and pasting change the draft as the owning hook does. */
function Harness({ draft: initial = localDraft, tier = "simple", ...rest }: HarnessProps) {
  const [draft, setDraft] = useState(initial);
  const validation = validateDraft(draft);
  return (
    <MCPServerEditor
      availableTargets={["auto", "config", "sidecar"]}
      draft={draft}
      entry={null}
      errors={validation.errors}
      initialTier={tier}
      isSaving={false}
      isValid={validation.valid}
      mode="create"
      onChange={updater => setDraft(updater)}
      onClose={() => undefined}
      onSave={() => undefined}
      onTargetChange={() => undefined}
      open
      saveError={null}
      scope="user"
      scopeChoice={{ value: "user", projectName: "compozy", onChange: () => undefined }}
      target="auto"
      vaultInventory={{ status: "ready", refs: ["vault:mcp/user/shared/GITHUB_TOKEN"] }}
      {...rest}
    />
  );
}

/** Nothing chosen yet: one field takes a URL or a command. */
export const AddEmpty: Story = {
  args: {},
  render: () => <Harness draft={emptyDraft("stdio")} />,
};

/** A remote URL: the preview says sign-in, if any, comes after adding. */
export const AddRemote: Story = {
  args: {},
  render: () => <Harness draft={remoteDraft} />,
};

/** A local command that needs a key: the key gets its own section. */
export const AddLocalNeedsKey: Story = {
  args: {},
  render: () => <Harness />,
};

/** Pasting the mcpServers JSON from a README fills in the form. */
export const AddFromJSON: Story = {
  args: {},
  tags: ["play-fn"],
  render: () => <Harness draft={emptyDraft("stdio")} />,
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByRole("button", { name: "Paste JSON instead" }));
    fireEvent.change(body.getByLabelText("Config from the docs"), {
      target: {
        value: JSON.stringify(
          {
            mcpServers: {
              github: {
                command: "npx",
                args: ["-y", "@modelcontextprotocol/server-github"],
                env: { GITHUB_TOKEN: "" },
              },
            },
          },
          null,
          2
        ),
      },
    });
    await expect(
      body.getByText("Found 1 server in the JSON and filled in the rest.")
    ).toBeVisible();
  },
};

/** More options open on a local program: arguments, settings, key names, where it's saved. */
export const MoreOptionsLocal: Story = {
  args: {},
  render: () => <Harness tier="advanced" />,
};

/** More options on a remote server: the OAuth choice replaces the process fields. */
export const MoreOptionsRemote: Story = {
  args: {},
  render: () => <Harness draft={remoteDraft} tier="advanced" />,
};

/** Editing locks the name and the scope; Remove sits on the footer's leading edge. */
export const Edit: Story = {
  args: {},
  render: () => (
    <Harness
      draft={configuredSecretDraft}
      mode="edit"
      onRemove={() => undefined}
      scopeChoice={{ value: "user", projectName: "compozy", disabled: true }}
    />
  ),
};

/** A Vault-bound key shows only the reference it binds. */
export const KeyFromVault: Story = {
  args: {},
  render: () => (
    <Harness
      draft={{
        ...localDraft,
        secretEnv: [
          {
            key: "GITHUB_TOKEN",
            binding: {
              mode: "ref",
              existing: false,
              typedValue: "",
              vaultRef: "vault:mcp/user/shared/GITHUB_TOKEN",
            },
          },
        ],
      }}
    />
  ),
};

/** A rejected save keeps the draft in place. */
export const SaveError: Story = {
  args: {},
  render: () => <Harness saveError="The settings write rejected the MCP definition." />,
};
