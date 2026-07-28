import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { Button } from "../../button";
import { DetailInspector, type DetailInspectorTab } from "../detail-inspector";

const meta: Meta<typeof DetailInspector> = {
  title: "components/custom/DetailInspector",
  component: DetailInspector,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "320 px right-rail panel. Renders inline at viewport >= `inlineBreakpoint` (default 1440 px) and collapses into a right-anchored `<Sheet>` drawer below. Tabs + body slots are owned by the primitive; specialisations (`<SessionInspector>`, `<AgentInfoInspector>`) declare their own tabs + content.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const TABS: DetailInspectorTab[] = [
  { id: "summary", label: "Summary" },
  { id: "events", label: "Events" },
  { id: "params", label: "Params" },
];

function InspectorBody({ activeTab }: { activeTab: string }) {
  return (
    <div className="flex flex-col gap-2 p-4 text-[12.5px] text-muted">
      <p>Active tab: {activeTab}</p>
      <p>
        At ≥ 1440 px viewports this panel renders inline at 320 px. Resize the preview frame below
        1440 px to flip the primitive into Sheet drawer mode.
      </p>
    </div>
  );
}

/**
 * Inline — viewport at ≥ 1440 px renders the 320 px right-rail panel directly inside the layout.
 * The story uses a fixed-height shell so the inspector stretches against its parent.
 */
export const Inline: Story = {
  render: () => {
    function Harness() {
      const [activeTab, setActiveTab] = useState("summary");
      return (
        <div className="flex h-[480px] w-full overflow-hidden bg-canvas text-fg">
          <main className="flex flex-1 items-center justify-center text-[13px] text-muted">
            Main content area
          </main>
          <DetailInspector
            title="Session inspector"
            tabs={TABS}
            activeTab={activeTab}
            onTabChange={setActiveTab}
          >
            <InspectorBody activeTab={activeTab} />
          </DetailInspector>
        </div>
      );
    }
    return <Harness />;
  },
};

/**
 * Drawer — forces drawer mode by raising the breakpoint above the preview viewport. Pair with the
 * sandbox toggle below to open the Sheet drawer.
 */
export const Drawer: Story = {
  render: () => {
    function Harness() {
      const [open, setOpen] = useState(true);
      const [activeTab, setActiveTab] = useState("summary");
      return (
        <div className="flex h-[480px] w-full overflow-hidden bg-canvas p-4 text-fg">
          <main className="flex flex-1 flex-col items-center justify-center gap-3 text-[13px] text-muted">
            <p>Main content area</p>
            <Button type="button" variant="outline" size="sm" onClick={() => setOpen(true)}>
              Open inspector
            </Button>
          </main>
          <DetailInspector
            title="Session inspector"
            tabs={TABS}
            activeTab={activeTab}
            onTabChange={setActiveTab}
            inlineBreakpoint={Number.MAX_SAFE_INTEGER}
            open={open}
            onOpenChange={setOpen}
          >
            <InspectorBody activeTab={activeTab} />
          </DetailInspector>
        </div>
      );
    }
    return <Harness />;
  },
};
