import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import { VaultEditor } from "../vault-editor";
import type { VaultDraft, VaultEditorState } from "../../hooks/use-vault-page";

const meta: Meta<typeof VaultEditor> = {
  title: "systems/vault/components/VaultEditor",
  component: VaultEditor,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Vault create body on the shared settings editor shell: a reference section and a write-only value section. The overwrite notice appears only when the normalized ref already exists.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function draft(overrides: Partial<VaultDraft> = {}): VaultDraft {
  return {
    ref: "vault:providers/openai/api-key",
    kind: "api_key",
    secretValue: "",
    overwriteConfirmed: false,
    ...overrides,
  };
}

interface HarnessProps {
  initial: VaultDraft;
  refExists?: boolean;
  isSaving?: boolean;
  error?: string | null;
}

function VaultEditorHarness({ initial, refExists = false, isSaving, error }: HarnessProps) {
  const [editor, setEditor] = useState<VaultEditorState>({ mode: "create", draft: initial });
  const current = editor.mode === "create" ? editor.draft : initial;
  const canSave =
    current.ref.startsWith("vault:") &&
    current.secretValue.trim() !== "" &&
    (!refExists || current.overwriteConfirmed);

  return (
    <CenteredSurface className="flex-col gap-4">
      <VaultEditor
        canSave={canSave}
        editor={editor}
        error={error ?? null}
        isSaving={Boolean(isSaving)}
        onChange={updater =>
          setEditor(state =>
            state.mode === "create" ? { ...state, draft: updater(state.draft) } : state
          )
        }
        onClose={() => undefined}
        onSave={() => undefined}
        refExists={refExists}
      />
    </CenteredSurface>
  );
}

/** VC-08 capture target: a clean create against an unused reference. */
export const Default: Story = {
  args: {},
  render: () => <VaultEditorHarness initial={draft({ secretValue: "sk-live-example" })} />,
};

/** Duplicate reference, before confirmation: the write stays blocked. */
export const DuplicateRefUnconfirmed: Story = {
  args: {},
  render: () => (
    <VaultEditorHarness initial={draft({ secretValue: "sk-live-example" })} refExists />
  ),
};

/** Duplicate reference, after confirmation: the write is unblocked. */
export const DuplicateRefConfirmed: Story = {
  args: {},
  render: () => (
    <VaultEditorHarness
      initial={draft({ secretValue: "sk-live-example", overwriteConfirmed: true })}
      refExists
    />
  ),
};

/** Invalid reference prefix surfaces the row error and the shell alert. */
export const InvalidRef: Story = {
  args: {},
  render: () => <VaultEditorHarness initial={draft({ ref: "providers/openai/api-key" })} />,
};

/** Saving: the write input disables and the primary blocks duplicate submits. */
export const Saving: Story = {
  args: {},
  render: () => <VaultEditorHarness initial={draft({ secretValue: "sk-live-example" })} isSaving />,
};
