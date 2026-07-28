import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { KnowledgeDeleteDialog } from "@/systems/knowledge/components/knowledge-delete-dialog";

const meta: Meta<typeof KnowledgeDeleteDialog> = {
  title: "systems/knowledge/components/KnowledgeDeleteDialog",
  component: KnowledgeDeleteDialog,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  render: () => (
    <KnowledgeDeleteDialog
      filename="project-context.md"
      isPending={false}
      onConfirm={async () => {}}
      onOpenChange={() => undefined}
      open
      scope="workspace"
    />
  ),
};

export const PendingDelete: Story = {
  args: {},
  render: () => (
    <KnowledgeDeleteDialog
      filename="user-role.md"
      isPending
      onConfirm={async () => {}}
      onOpenChange={() => undefined}
      open
      scope="global"
    />
  ),
};

export const ConfirmSubmits: Story = {
  args: {},
  tags: ["play-fn"],
  render: () => {
    const onConfirm = fn();
    return (
      <KnowledgeDeleteDialog
        filename="project-context.md"
        isPending={false}
        onConfirm={onConfirm}
        onOpenChange={() => undefined}
        open
        scope="workspace"
      />
    );
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    await userEvent.type(
      await canvas.findByTestId("knowledge-delete-confirm-typing"),
      "project-context.md"
    );
    const confirm = await canvas.findByTestId("confirm-delete-memory-btn");
    await userEvent.click(confirm);
    await expect(confirm).toBeVisible();
  },
};
