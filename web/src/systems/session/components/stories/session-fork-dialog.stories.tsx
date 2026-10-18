import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";

import {
  deriveSourceSessionFixture,
  forkCutPreviewFixture,
  forkNativePreviewFixture,
  forkPointFixture,
  forkUnsettledPreviewFixture,
} from "../../mocks/derive-fixtures";
import {
  sessionDeriveHandlers,
  type SessionDeriveHandlerOptions,
} from "../../mocks/derive-handlers";
import { SessionForkDialog } from "../session-fork-dialog";

function deriveParameters(options: SessionDeriveHandlerOptions = {}) {
  return storybookMswParameters({ session: sessionDeriveHandlers(options) });
}

const meta: Meta<typeof SessionForkDialog> = {
  title: "systems/session/components/SessionForkDialog",
  component: SessionForkDialog,
  args: {
    source: deriveSourceSessionFixture,
    workspaceId: deriveSourceSessionFixture.workspace_id ?? "",
    point: null,
    open: true,
    onOpenChange: fn(),
    placement: { openInNewWindow: fn(), openInThisWindow: fn() },
  },
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Fork session (dialogs board §02). The agent is locked — another agent is a Continue. The fork point is the whole session or, from “Fork from here”, the clicked message and its turn; Change closes the dialog because the point is chosen on the message. The native-clone sentence appears only when the daemon's preview reports `native_fork_possible`.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

async function submitWhenMeasured(canvasElement: HTMLElement) {
  const body = within(canvasElement.ownerDocument.body);
  const submit = await body.findByTestId("session-fork-submit");
  await waitFor(() => expect(submit).toBeEnabled());
  await userEvent.click(submit);
  return body;
}

/** VC-11: opened from a menu — the whole session. */
export const WholeSession: Story = { parameters: deriveParameters() };

/** VC-12: opened by "Fork from here" — through the quoted message, with Change. */
export const FromMessage: Story = {
  args: { point: forkPointFixture },
  parameters: deriveParameters({ preview: forkCutPreviewFixture }),
};

/** VC-13: the daemon reports the native clone is possible for this source. */
export const NativeClone: Story = {
  parameters: deriveParameters({ preview: forkNativePreviewFixture }),
};

/** VC-14: the cut's turn has not settled — the line says so and Fork session stays disabled. */
export const TurnInProgress: Story = {
  args: { point: forkPointFixture },
  parameters: deriveParameters({ preview: forkUnsettledPreviewFixture }),
};

/** VC-15: the transcript moved after opening; the daemon refused the fences. Cancel reads Close. */
export const FenceConflict: Story = {
  args: { point: forkPointFixture },
  parameters: deriveParameters({
    preview: forkCutPreviewFixture,
    result: {
      status: 409,
      code: "session_fence_conflict",
      error: "transcript changed since the fences were read",
    },
  }),
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = await submitWhenMeasured(canvasElement);
    await expect(await body.findByTestId("session-fork-submit-error")).toHaveTextContent(
      "Transcript changed — reopen to fork from the current state."
    );
  },
};

/** Measuring: the preview is in flight and Fork session waits for it. */
export const Measuring: Story = { parameters: deriveParameters({ preview: "pending" }) };

/** Submitted and waiting on the daemon — `Starting the new session…`, close hidden. */
export const Pending: Story = {
  parameters: deriveParameters({ result: "pending" }),
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = await submitWhenMeasured(canvasElement);
    await expect(await body.findByTestId("session-fork-pending-status")).toBeVisible();
  },
};
