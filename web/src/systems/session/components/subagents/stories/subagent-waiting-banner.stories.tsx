import type { ReactNode } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { ArrowUp } from "lucide-react";
import { fn } from "storybook/test";

import { Toaster } from "@compozy/ui";

import { SubagentWaitingBanner } from "../subagent-waiting-banner";
import { cardStates, roster } from "./subagent-story-fixtures";

const three = [
  { ...cardStates.running, title: "Audit payment webhooks for retry safety" },
  { ...cardStates.waiting, status: "running" as const },
  { ...cardStates.completed, status: "running" as const, settled_at: null },
];
const five = [
  ...three,
  { ...roster[3]!, id: "e2e", title: "Run the checkout e2e suite" },
  { ...roster[4]!, id: "profile" },
];

/** Lean stand-in for the composer frame: the banner docks on its top edge. */
function ComposerDock({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex w-180 flex-col gap-2">
      <span className="font-mono text-mono-id text-faint">{label}</span>
      <div>
        {children}
        <div className="relative z-1 flex flex-col rounded-xl border border-line-strong bg-canvas-soft shadow-card">
          <span className="min-h-11.5 px-3.5 pt-3 pb-1 text-small-body text-faint">
            Message dev-loop…
          </span>
          <span className="flex justify-end px-2 pb-2">
            <span className="grid size-7 place-items-center rounded-pill bg-primary text-primary-foreground">
              <ArrowUp aria-hidden="true" className="size-3.5" />
            </span>
          </span>
        </div>
      </div>
    </div>
  );
}

const never = () => new Promise<never>(() => undefined);
const failing = () =>
  new Promise<never>((_, reject) => setTimeout(() => reject(new Error("stop failed")), 600));

/** Composer VC-01/02: one, many, stopping, stop failed. */
function BannerStates() {
  return (
    <div className="flex flex-col gap-8 p-8">
      <Toaster />
      <ComposerDock label="live · 5 subagents · wake pending">
        <SubagentWaitingBanner
          subagents={five}
          wakePending
          onOpen={fn()}
          onShowAll={fn()}
          onStop={never}
        />
      </ComposerDock>
      <ComposerDock label="one · the title carries the name">
        <SubagentWaitingBanner subagents={[three[2]!]} onOpen={fn()} onStop={never} />
      </ComposerDock>
      <ComposerDock label="many · three names, no and N more">
        <SubagentWaitingBanner subagents={three} onOpen={fn()} onStop={never} />
      </ComposerDock>
      <ComposerDock label="stopping · disabled, names stay">
        <SubagentWaitingBanner subagents={three} onOpen={fn()} stopping />
      </ComposerDock>
      <ComposerDock label="stop failed · press Stop, toast">
        <SubagentWaitingBanner subagents={three} onOpen={fn()} onStop={failing} />
      </ComposerDock>
    </div>
  );
}

const meta: Meta = {
  title: "systems/session/components/subagents/WaitingBanner",
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const States: Story = { render: () => <BannerStates /> };
