import type { ReactNode } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { HoverCard, HoverCardContent, HoverCardTrigger } from "@compozy/ui";

import { SessionInspectorSubagentsSection } from "../session-inspector-subagents-section";
import { SubagentCard } from "../subagent-card";
import { SubagentGroup } from "../subagent-group";
import { SubagentHoverContent } from "../subagent-hover-content";
import type { SubagentView } from "../types";
import { isolatedStates, secondsAgo } from "./subagent-story-fixtures";

const onOpen = fn();

const fanOut: SubagentView[] = [
  isolatedStates.prOpen,
  isolatedStates.running,
  { ...isolatedStates.prUnknown, title: "Cap webhook retries" },
];

function Cell({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <span className="font-mono text-mono-id text-faint">{label}</span>
      {children}
    </div>
  );
}

/** Agent-collaboration VC-06: the card and trail in every isolated state, plus shared unchanged. */
function CardStates() {
  const cells: [string, SubagentView][] = [
    ["running isolated · branch only", isolatedStates.running],
    ["settled · PR open", isolatedStates.prOpen],
    ["settled · PR draft", isolatedStates.prDraft],
    ["settled · PR merged", isolatedStates.prMerged],
    ["settled · PR closed", isolatedStates.prClosed],
    ["settled · no PR (none)", isolatedStates.noPr],
    ["settled · PR status unknown", isolatedStates.prUnknown],
    ["failed isolated · branch kept", isolatedStates.failed],
    ["shared subagent · unchanged", isolatedStates.shared],
  ];
  return (
    <div className="grid max-w-300 grid-cols-2 gap-x-8 gap-y-6 p-8">
      {cells.map(([label, subagent]) => (
        <Cell key={label} label={label}>
          <SubagentCard subagent={subagent} onOpen={onOpen} />
        </Cell>
      ))}
    </div>
  );
}

function OpenHover({ label, subagent }: { label: string; subagent: SubagentView }) {
  return (
    <div className="h-80">
      <HoverCard defaultOpen>
        <HoverCardTrigger render={<span className="font-mono text-mono-id text-faint" />}>
          {label}
        </HoverCardTrigger>
        <HoverCardContent>
          <SubagentHoverContent subagent={subagent} />
        </HoverCardContent>
      </HoverCard>
    </div>
  );
}

/** Agent-collaboration VC-07: hover facts. Unobserved facts are absent; unknown is not none. */
function HoverStates() {
  return (
    <div className="grid max-w-5xl grid-cols-3 gap-x-8 p-8">
      <OpenHover label="running · static facts only" subagent={isolatedStates.running} />
      <OpenHover label="settled · PR open · clean" subagent={isolatedStates.prOpen} />
      <OpenHover label="settled · merged" subagent={isolatedStates.prMerged} />
      <OpenHover label="dirty · no PR" subagent={isolatedStates.noPr} />
      <OpenHover label="PR status unknown" subagent={isolatedStates.prUnknown} />
      <OpenHover label="git read failed · PR still shown" subagent={isolatedStates.gitReadFailed} />
    </div>
  );
}

/** Agent-collaboration VC-07: isolated rows grow a branch line; shared rows stay one line. */
function RosterStates() {
  return (
    <div className="w-80 p-6">
      <SessionInspectorSubagentsSection
        subagents={[
          { ...isolatedStates.failed, created_at: secondsAgo(900) },
          {
            ...isolatedStates.running,
            title: "Cap webhook retries",
            created_at: secondsAgo(800),
          },
          { ...isolatedStates.prMerged, created_at: secondsAgo(700) },
          { ...isolatedStates.noPr, created_at: secondsAgo(600) },
          { ...isolatedStates.shared, created_at: secondsAgo(500) },
        ]}
        onOpen={onOpen}
        onStop={async () => undefined}
        defaultPreviousOpen
      />
    </div>
  );
}

const meta: Meta = {
  title: "systems/session/components/subagents/Worktree",
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Agent-collaboration §01: one turn, three branches, PRs as they land. */
export const FanOut: Story = {
  render: () => (
    <div className="grid grid-cols-[minmax(0,1fr)_320px] gap-8 p-8">
      <SubagentGroup subagents={fanOut} onOpen={onOpen} defaultOpen />
      <SessionInspectorSubagentsSection subagents={fanOut} onOpen={onOpen} />
    </div>
  ),
};

export const Cards: Story = { render: () => <CardStates /> };
export const Hover: Story = { render: () => <HoverStates /> };
export const Roster: Story = { render: () => <RosterStates /> };
