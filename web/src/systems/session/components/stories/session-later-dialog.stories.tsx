import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { SessionLaterDialog } from "../session-later-dialog";
import { BULK_SELECTED_SESSIONS } from "./session-bulk-fixtures";

const meta: Meta<typeof SessionLaterDialog> = {
  title: "systems/session/components/SessionLaterDialog",
  component: SessionLaterDialog,
  parameters: { layout: "centered" },
  args: {
    open: true,
    isMoving: false,
    onOpenChange: fn(),
    onConfirm: fn(),
    sessions: [BULK_SELECTED_SESSIONS[0]!],
  },
};
export default meta;
type Story = StoryObj<typeof meta>;

/** One session: an optional one-line reason and an optional return date. */
export const Single: Story = {};

/** A bulk move names how many sessions stop. */
export const Selection: Story = { args: { sessions: BULK_SELECTED_SESSIONS } };

/** Parking stops the session tree first, so the confirm holds until the daemon answers. */
export const Moving: Story = { args: { isMoving: true } };
