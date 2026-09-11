import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { SessionDeleteDialog } from "../session-delete-dialog";
import { BULK_SELECTED_SESSIONS } from "./session-bulk-fixtures";

const meta: Meta<typeof SessionDeleteDialog> = {
  title: "systems/session/components/SessionDeleteDialog",
  component: SessionDeleteDialog,
  parameters: { layout: "centered" },
  args: { open: true, isDeleting: false, onOpenChange: fn(), onConfirm: fn(), onRetry: fn() },
};
export default meta;
type Story = StoryObj<typeof meta>;

export const Single: Story = { args: { session: BULK_SELECTED_SESSIONS[0] } };
export const ConfirmSet: Story = { args: { sessions: BULK_SELECTED_SESSIONS } };
export const DeletingSet: Story = {
  args: {
    sessions: BULK_SELECTED_SESSIONS,
    isDeleting: true,
    results: [
      { id: "issue-613", status: "done" },
      { id: "integrate", status: "running" },
      { id: "issue-606", status: "pending" },
    ],
  },
};
export const PartialFailure: Story = {
  args: {
    sessions: BULK_SELECTED_SESSIONS,
    results: [
      { id: "issue-613", status: "done" },
      { id: "integrate", status: "done" },
      { id: "issue-606", status: "failed", error: "session is locked by another operation." },
    ],
  },
};
