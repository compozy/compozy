import type { Meta, StoryObj } from "@storybook/react-vite";

import { LoopNodeControlDialog } from "../run-page/loop-node-control-dialog";
import { LoopQuarantineSheet } from "../run-page/loop-quarantine-sheet";
import { projectNodeLifecycles } from "../../lib/loop-node-lifecycle";
import { quarantinedScenario } from "./loop-run-lifecycle-fixtures";

/**
 * Visual Contract capture target for the quarantine entry sheet (VC-R4). The
 * node is projected from the same durable payloads the daemon returns, so a
 * story that reads right is evidence the production path reads right.
 */
function quarantinedNode() {
  const scenario = quarantinedScenario();
  const nodes = projectNodeLifecycles({
    controls: scenario.nodeControls,
    waits: scenario.waits,
    generations: scenario.generations,
    runGeneration: 2,
  });
  const node = nodes.find(candidate => candidate.quarantined);
  if (!node) throw new Error("quarantine fixture lost its quarantined node");
  return node;
}

const meta: Meta<typeof LoopQuarantineSheet> = {
  title: "systems/loops/components/LoopQuarantineSheet",
  component: LoopQuarantineSheet,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const QuarantineEntry: Story = {
  args: {},
  render: () => (
    <div className="h-dvh bg-canvas">
      <LoopQuarantineSheet
        node={quarantinedNode()}
        onOpenChange={() => {}}
        onVerb={() => {}}
        open
        runId="r-7c4e19"
      />
    </div>
  ),
};

/** The requeue confirm, nested over the sheet, restating the quarantine state. */
export const RequeueConfirmation: Story = {
  args: {},
  render: () => {
    const node = quarantinedNode();
    return (
      <div className="h-dvh bg-canvas">
        <LoopQuarantineSheet
          node={node}
          onOpenChange={() => {}}
          onVerb={() => {}}
          open
          runId="r-7c4e19"
        >
          <LoopNodeControlDialog
            onConfirm={() => {}}
            onOpenChange={() => {}}
            request={{ verb: "requeue", node }}
          />
        </LoopQuarantineSheet>
      </div>
    );
  },
};

/** A node that left quarantine between renders: the requeue verb disappears. */
export const QuarantineEntryAfterRequeue: Story = {
  args: {},
  render: () => (
    <div className="h-dvh bg-canvas">
      <LoopQuarantineSheet
        node={{ ...quarantinedNode(), quarantined: false, state: null, parked: false }}
        onOpenChange={() => {}}
        onVerb={() => {}}
        open
        runId="r-7c4e19"
      />
    </div>
  ),
};
