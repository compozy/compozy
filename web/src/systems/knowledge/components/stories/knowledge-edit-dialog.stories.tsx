import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { KnowledgeEditDialog } from "@/systems/knowledge/components/knowledge-edit-dialog";

const meta: Meta<typeof KnowledgeEditDialog> = {
  title: "systems/knowledge/components/KnowledgeEditDialog",
  component: KnowledgeEditDialog,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const initialContent = [
  "# Operator Style",
  "",
  "Lead with the fact pattern, then the decision, then the next concrete action.",
].join("\n");

const identity = {
  filename: "operator-style.md",
  name: "operator-style",
  type: "user",
} as const;

/** Resting edit state: locked identity above the two editable fields. */
export const Default: Story = {
  args: {},
  render: () => (
    <KnowledgeEditDialog
      {...identity}
      initialContent={initialContent}
      initialDescription="Northstar guidance for concise, accountable operator communication."
      isPending={false}
      onConfirm={async () => {}}
      onOpenChange={() => undefined}
      open
      scope="global"
    />
  ),
};

/** Saving: the primary action swaps to a spinner and blocks duplicate submits. */
export const PendingSave: Story = {
  args: {},
  render: () => (
    <KnowledgeEditDialog
      {...identity}
      initialContent={initialContent}
      initialDescription=""
      isPending
      onConfirm={async () => {}}
      onOpenChange={() => undefined}
      open
      scope="global"
    />
  ),
};

/** Save error: the draft is retained and the failure is reported above the footer. */
export const RejectedByPolicy: Story = {
  args: {},
  render: () => (
    <KnowledgeEditDialog
      {...identity}
      error="Edit rejected by policy: invisible Unicode in content"
      initialContent={initialContent}
      initialDescription=""
      isPending={false}
      onConfirm={async () => {}}
      onOpenChange={() => undefined}
      open
      scope="global"
    />
  ),
};

/** VC-05 capture target: the immutable name/type identity block. */
export const ImmutableIdentityLock: Story = {
  args: {},
  render: () => (
    <KnowledgeEditDialog
      {...identity}
      initialContent={initialContent}
      initialDescription="Northstar guidance for concise, accountable operator communication."
      isPending={false}
      onConfirm={async () => {}}
      onOpenChange={() => undefined}
      open
      scope="workspace"
    />
  ),
};

export const ConfirmSubmits: Story = {
  args: {},
  tags: ["play-fn"],
  render: () => {
    const onConfirm = fn();
    return (
      <KnowledgeEditDialog
        {...identity}
        initialContent={initialContent}
        initialDescription="Northstar guidance"
        isPending={false}
        onConfirm={onConfirm}
        onOpenChange={() => undefined}
        open
        scope="global"
      />
    );
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    const editor = await canvas.findByTestId("knowledge-edit-content");
    await userEvent.type(editor, "\n\nFollow up with the next concrete metric.");
    const confirm = await canvas.findByTestId("confirm-edit-memory-btn");
    await expect(confirm).toBeVisible();
  },
};
