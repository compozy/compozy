import type { ReactNode } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { FileText, Search } from "lucide-react";
import { fn } from "storybook/test";

import { StateGlyph, ToolCallRow } from "@compozy/ui";

import { SubagentCard } from "../subagent-card";
import { SubagentGroup } from "../subagent-group";
import { SubagentOriginDivider } from "../subagent-origin-divider";
import {
  cardStates,
  nativeRunning,
  nativeSettled,
  subagentFixture,
} from "./subagent-story-fixtures";

function Cell({ label, children, wide }: { label: string; children: ReactNode; wide?: boolean }) {
  return (
    <div className={wide ? "col-span-2 flex flex-col gap-2" : "flex flex-col gap-2"}>
      <span className="font-mono text-mono-id text-faint">{label}</span>
      {children}
    </div>
  );
}

const onOpen = fn();

/** Transcript VC-02: the card in every state the roster can report. */
function CardStates() {
  return (
    <div className="grid max-w-5xl grid-cols-2 gap-x-6 gap-y-5 p-8">
      <Cell label="queued · no progress yet">
        <SubagentCard subagent={cardStates.queued} onOpen={onOpen} />
      </Cell>
      <Cell label="running · no progress">
        <SubagentCard subagent={cardStates.running} onOpen={onOpen} />
      </Cell>
      <Cell label="running · with progress">
        <SubagentCard subagent={cardStates.runningProgress} onOpen={onOpen} />
      </Cell>
      <Cell label="waiting for you">
        <SubagentCard subagent={cardStates.waiting} onOpen={onOpen} />
      </Cell>
      <Cell label="completed · result first line · frozen">
        <SubagentCard subagent={cardStates.completed} onOpen={onOpen} />
      </Cell>
      <Cell label="failed · line 2 destructive">
        <SubagentCard subagent={cardStates.failed} onOpen={onOpen} />
      </Cell>
      <Cell label="canceled">
        <SubagentCard subagent={cardStates.canceled} onOpen={onOpen} />
      </Cell>
      <Cell label="interrupted">
        <SubagentCard subagent={cardStates.interrupted} onOpen={onOpen} />
      </Cell>
      <Cell label="unknown provider mark · bot glyph">
        <SubagentCard subagent={cardStates.unknownProvider} onOpen={onOpen} />
      </Cell>
      <Cell label="stale after reconnect · no ticking">
        <SubagentCard subagent={cardStates.stale} onOpen={onOpen} stale />
      </Cell>
      <Cell label="title at 72 characters" wide>
        <SubagentCard subagent={cardStates.longTitle} onOpen={onOpen} />
      </Cell>
      <Cell label="provider-native · no child session · no chevron" wide>
        <SubagentCard subagent={nativeRunning} onOpen={onOpen} />
      </Cell>
    </div>
  );
}

function NativeRows({ settled = false }: { settled?: boolean }) {
  return (
    <>
      <ToolCallRow
        icon={Search}
        status="success"
        toolName="Searched"
        preview="RefundReason in internal/"
      />
      <ToolCallRow
        icon={Search}
        status="success"
        toolName="Searched"
        preview="refund_reason in migrations/"
      />
      <ToolCallRow
        icon={FileText}
        status={settled ? "success" : "running"}
        toolName={settled ? "Read" : "Reading"}
        preview="internal/store/refunds.go"
      />
    </>
  );
}

/** Transcript VC-05: provider-native work nests inside its card. */
function NativeStates() {
  return (
    <div className="grid max-w-5xl grid-cols-2 gap-x-6 gap-y-5 p-8">
      <Cell label="collapsed · default">
        <SubagentCard subagent={nativeRunning} nested={<NativeRows />} />
      </Cell>
      <Cell label="expanded · live inner rows">
        <SubagentCard
          subagent={{ ...nativeRunning, progress: "Reading internal/store/refunds.go" }}
          nested={<NativeRows />}
          defaultExpanded
        />
      </Cell>
      <Cell label="settled · result on line 2 · no drill-in" wide>
        <SubagentCard subagent={nativeSettled} nested={<NativeRows settled />} />
      </Cell>
    </div>
  );
}

const reviewer = { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "medium" };
const groupRunning = [
  cardStates.runningProgress,
  subagentFixture({
    id: "g2",
    title: "Review PR #812 for N+1 queries",
    runtime: reviewer,
    elapsed: 40,
  }),
  subagentFixture({ id: "g3", title: "Draft release notes for v2", elapsed: 12 }),
];

/** Transcript VC-03: same-turn delegations fold into one disclosure. */
function GroupStates() {
  return (
    <div className="grid max-w-5xl grid-cols-2 gap-x-6 gap-y-5 p-8">
      <Cell label="all running · collapsed">
        <SubagentGroup subagents={groupRunning} onOpen={onOpen} />
      </Cell>
      <Cell label="mixed · live + done">
        <SubagentGroup
          subagents={[groupRunning[0]!, groupRunning[1]!, cardStates.completed]}
          onOpen={onOpen}
        />
      </Cell>
      <Cell label="needs you">
        <SubagentGroup
          subagents={[groupRunning[0]!, cardStates.waiting, cardStates.completed]}
          onOpen={onOpen}
        />
      </Cell>
      <Cell label="any failed · danger summary">
        <SubagentGroup
          subagents={[groupRunning[0]!, cardStates.completed, cardStates.failed]}
          onOpen={onOpen}
        />
      </Cell>
      <Cell label="overflow · 3 avatars + +N">
        <SubagentGroup
          subagents={[
            ...groupRunning,
            cardStates.unknownProvider,
            cardStates.queued,
            cardStates.completed,
            { ...cardStates.completed, id: "done-2" },
          ]}
          onOpen={onOpen}
        />
      </Cell>
      <Cell label="all done · collapsed + settled → dimmed">
        <SubagentGroup
          subagents={[
            cardStates.completed,
            {
              ...cardStates.completed,
              id: "done-b",
              title: "Review PR #812 for N+1 queries",
              runtime: reviewer,
            },
          ]}
          onOpen={onOpen}
        />
      </Cell>
      <Cell label="expanded · panel lists cards" wide>
        <SubagentGroup
          subagents={[
            { ...cardStates.running, progress: "Reading internal/payments/webhook.go" },
            cardStates.failed,
          ]}
          onOpen={onOpen}
          defaultOpen
        />
      </Cell>
    </div>
  );
}

/** Transcript VC-07: the child transcript's first hairline. */
function DividerStates() {
  return (
    <div className="flex max-w-3xl flex-col gap-8 p-8">
      <SubagentOriginDivider
        parent={{ id: "sess-7f3a2c11d09e4b58", title: "Ship checkout v2" }}
        onOpenParent={fn()}
      />
      <SubagentOriginDivider parent={null} onOpenParent={fn()} />
    </div>
  );
}

/** Subagent transcript surfaces: the live card, the same-turn group, and the child divider. */
const meta: Meta = {
  title: "systems/session/components/subagents/Transcript",
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Cards: Story = { render: () => <CardStates /> };
export const ProviderNative: Story = { render: () => <NativeStates /> };
export const Groups: Story = { render: () => <GroupStates /> };
export const OriginDivider: Story = { render: () => <DividerStates /> };

/** Transcript VC-01: a live turn delegating three subagents, then text. */
export const LiveTurn: Story = {
  render: () => (
    <div className="flex max-w-180 flex-col gap-3 p-8 text-transcript-message text-fg">
      <p>I'll fan this out: one audit, one review, one set of notes.</p>
      <SubagentGroup subagents={groupRunning} onOpen={onOpen} defaultOpen />
      <p className="flex items-center gap-2 text-subtle">
        <StateGlyph size="sm" state="running" />
        While those run, I'll check the refund edge cases myself.
      </p>
    </div>
  ),
};
