import type { Meta, StoryObj } from "@storybook/react-vite";
import { KeyRound, Settings2, Trash2 } from "lucide-react";
import { fn } from "storybook/test";

import {
  ConfirmDialog,
  Field,
  FieldHeader,
  FieldLabel,
  FormSection,
  HelpTip,
  Input,
  Pill,
} from "@agh/ui";

import { ModalSettingsFieldRow } from "../settings-field-row";
import { SettingsEditorDialog } from "../settings-editor-dialog";

const meta: Meta<typeof SettingsEditorDialog> = {
  title: "systems/settings/components/SettingsDialogs",
  component: SettingsEditorDialog,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "`SettingsEditorDialog` pins the shared modal shell — ruled `EntityDialogHeader`, host size token, feedback strip, and `EntityDialogFooter` with its consequence hint — so vault and sandbox inherit chrome without per-page header forks. These stories exercise the *shell*; the real bodies live in `VaultEditor`, `MCPServerEditor`, and `ProviderDetailDialog`. Bodies here use the same `FormSection` + `ModalSettingsFieldRow` grammar production uses — never the settings-page row grammar, which belongs to routes.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Shell with metadata and a warning in the feedback strip. */
export const Editor: Story = {
  args: {},
  render: () => (
    <SettingsEditorDialog
      open
      canSave
      description="Update command and model defaults for this provider overlay."
      eyebrow="Settings · Provider"
      hint="Saved overlays apply to new sessions in this workspace."
      icon={Settings2}
      isSaving={false}
      metadata={<Pill tone="info">workspace override</Pill>}
      mode="edit"
      onOpenChange={fn()}
      onSave={fn()}
      size="md"
      slug="providers"
      title="Edit provider"
      warnings={["Changing the command requires a daemon restart."]}
    >
      <FormSection
        help="Both fall back to the built-in provider configuration when left empty."
        title="Provider basics"
      >
        <ModalSettingsFieldRow
          control={<Input className="font-mono" defaultValue="codex" />}
          data-testid="settings-providers-command"
          help="The executable AGH launches for this provider."
          label="Command"
        />
        <ModalSettingsFieldRow
          control={<Input className="font-mono" defaultValue="gpt-5.4" />}
          data-testid="settings-providers-model"
          help="Used by any session that does not pick a model of its own."
          label="Default model"
        />
      </FormSection>
    </SettingsEditorDialog>
  ),
};

/** Destructive confirmation runs on the neutral `ConfirmDialog` shell, not this one. */
export const Delete: Story = {
  args: {},
  render: () => (
    <ConfirmDialog
      open
      cancelLabel="Cancel"
      confirmIcon={Trash2}
      confirmLabel="Delete"
      contentProps={{ "data-testid": "settings-providers-delete" }}
      description="This removes the workspace override; built-in provider defaults remain available."
      isPending={false}
      note="The provider falls back to the built-in config after deletion."
      onConfirm={fn()}
      onOpenChange={fn()}
      title="Delete provider overlay"
    />
  ),
};

/** A save error blocks the primary and states what failed. */
export const SaveError: Story = {
  args: {},
  render: () => (
    <SettingsEditorDialog
      open
      canSave={false}
      error="Server command failed validation."
      eyebrow="System · MCP server"
      icon={KeyRound}
      isSaving={false}
      mode="create"
      onOpenChange={fn()}
      onSave={fn()}
      size="md"
      slug="mcp"
      title="Add MCP server"
    >
      <FormSection title="How does it run?">
        <Field data-invalid>
          <FieldLabel htmlFor="mcp-name">Name</FieldLabel>
          <Input aria-invalid className="font-mono" id="mcp-name" />
        </Field>
        <Field>
          <FieldLabel htmlFor="mcp-command">Command</FieldLabel>
          <Input
            className="font-mono"
            id="mcp-command"
            placeholder="npx -y @modelcontextprotocol/server-github"
          />
        </Field>
      </FormSection>
    </SettingsEditorDialog>
  ),
};

/** The `sm` host with a help tip beside a section title. */
export const CompactHost: Story = {
  args: {},
  render: () => (
    <SettingsEditorDialog
      open
      canSave
      description="Stores a write-only secret value and returns redacted metadata."
      eyebrow="System · Vault"
      hint={
        <>
          Bind it from providers, bridges, or sandboxes as{" "}
          <b className="font-medium text-muted">vault:&lt;reference&gt;</b>.
        </>
      }
      icon={KeyRound}
      isSaving={false}
      mode="create"
      onOpenChange={fn()}
      onSave={fn()}
      saveLabel="Store secret"
      size="sm"
      slug="vault"
      title="Add vault secret"
    >
      <FormSection
        help="Providers, bridges, and sandboxes bind this value by its reference."
        title="The reference"
      >
        <Field>
          <FieldHeader>
            <FieldLabel htmlFor="vault-ref">Secret reference</FieldLabel>
            <HelpTip label="About secret reference">
              Must start with <b>vault:</b>. This is the only part of a secret that stays readable.
            </HelpTip>
          </FieldHeader>
          <Input
            className="font-mono"
            defaultValue="vault:providers/openai/api-key"
            id="vault-ref"
          />
        </Field>
      </FormSection>
    </SettingsEditorDialog>
  ),
};
