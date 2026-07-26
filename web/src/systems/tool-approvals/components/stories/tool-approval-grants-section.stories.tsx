import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { fn } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { StorybookWorkspaceSetup } from "@/storybook/route-story-meta";
import { ToolApprovalGrantsSection } from "@/systems/tool-approvals";
import {
  emptyToolApprovalGrantsResponseFixture,
  toolApprovalGrantsResponseFixture,
} from "@/systems/tool-approvals/mocks";

import { ToolApprovalGrantSetDialog } from "../tool-approval-grant-set-dialog";

function Canvas() {
  return (
    <>
      <StorybookWorkspaceSetup />
      <div className="mx-auto max-w-3xl p-6">
        <ToolApprovalGrantsSection />
      </div>
    </>
  );
}

const widerDecisionActions = {
  change: fn(),
  openChange: fn(),
  submit: fn(),
};

function WiderDecisionCanvas() {
  return (
    <>
      <Canvas />
      <ToolApprovalGrantSetDialog
        canSubmit
        draft={{
          toolId: "agh__workspace_list",
          decision: "allow",
          scope: "agent",
          agentName: "claude-code",
        }}
        error={null}
        isPending={false}
        onChange={widerDecisionActions.change}
        onOpenChange={widerDecisionActions.openChange}
        onSubmit={widerDecisionActions.submit}
        open
      />
    </>
  );
}

const meta: Meta<typeof ToolApprovalGrantsSection> = {
  title: "systems/tool-approvals/components/ToolApprovalGrantsSection",
  component: ToolApprovalGrantsSection,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Permissions -> Remembered decisions: explicit wider native-tool decisions and prompt-origin exact decisions for the active workspace, with per-row revoke.",
      },
    },
  },
  render: () => <Canvas />,
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Populated -- agent-wide, tool-wide, and exact decisions from daemon truth. */
export const Populated: Story = { args: {} };

/** Empty -- no remembered decisions for the active workspace yet. */
export const Empty: Story = {
  args: {},
  parameters: storybookMswParameters({
    "tool-approvals": [
      aghApiMock.get("/api/tool-approval-grants", () =>
        HttpResponse.json(emptyToolApprovalGrantsResponseFixture)
      ),
    ],
  }),
};

/** Load error -- the list failed to load; the retry affordance is shown. */
export const LoadError: Story = {
  args: {},
  parameters: storybookMswParameters({
    "tool-approvals": [
      aghApiMock.get("/api/tool-approval-grants", () =>
        HttpResponse.json({ error: "daemon unavailable" }, { status: 503 })
      ),
    ],
  }),
};

/** Loading -- the list is still resolving. */
export const Loading: Story = {
  args: {},
  parameters: storybookMswParameters({
    "tool-approvals": [
      aghApiMock.get("/api/tool-approval-grants", async () => {
        await delay("infinite");
        return HttpResponse.json(toolApprovalGrantsResponseFixture);
      }),
    ],
  }),
};

/** Wider decision form -- scope and decision remain explicit operator choices. */
export const WiderDecisionForm: Story = {
  args: {},
  render: () => <WiderDecisionCanvas />,
};
