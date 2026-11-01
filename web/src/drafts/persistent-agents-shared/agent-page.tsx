/** The agent page in the Agents app: head with crumbs, focused editors and the hub all share it. */
import { Users2 } from "lucide-react";
import type { ReactNode } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Pill,
  PageContent,
  TopbarOverflowIcon,
  type TopbarSlotValue,
} from "@compozy/ui";

import { goToState } from "./go-to-state";

import { DraftShell, DraftUnsavedPill } from "./draft-shell";
import type { DraftAgent } from "./fixtures";

export function KindPill({ agent }: { agent: Pick<DraftAgent, "kind"> }) {
  return (
    <Pill size="sm" tone="neutral">
      {agent.kind === "persistent" ? "Persistent" : "Ephemeral"}
    </Pill>
  );
}

export function AgentOverflow() {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="More agent actions"
        render={<Button size="icon-sm" type="button" variant="quiet" />}
      >
        <TopbarOverflowIcon aria-hidden="true" className="size-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem>Duplicate</DropdownMenuItem>
        <DropdownMenuItem variant="destructive">Delete…</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export interface AgentAppPageProps {
  agent: DraftAgent;
  /** The focused editor's name; omitted on the hub. */
  leaf?: string;
  hubState?: string;
  actions?: ReactNode;
  children: ReactNode;
  width?: string;
  sessionBadge?: number;
}

/** One location of the Agents window: `Agents / Orchestrator / Heartbeat`. */
export function AgentAppPage({
  agent,
  leaf,
  hubState = "persistent",
  actions,
  children,
  width = "max-w-3xl",
  sessionBadge,
}: AgentAppPageProps) {
  const crumbs = [
    { id: "agents", label: "Agents", onSelect: () => undefined },
    ...(leaf ? [{ id: agent.name, label: agent.label, onSelect: () => goToState(hubState) }] : []),
  ];
  const topbar: TopbarSlotValue = {
    crumbs,
    crumb: leaf ?? agent.label,
    onBack: () => (leaf ? goToState(hubState) : undefined),
    actions,
    overflow: leaf ? undefined : <AgentOverflow />,
  };
  return (
    <DraftShell
      app="agents"
      glyph={<Users2 />}
      sessionBadge={sessionBadge}
      title={leaf ?? agent.label}
      topbar={topbar}
    >
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
        <PageContent className={width}>{children}</PageContent>
      </div>
    </DraftShell>
  );
}

/** The dirty-state head actions: the unsaved count, Discard, Save. */
export function SaveActions({ unsaved, hub = "persistent" }: { unsaved: number; hub?: string }) {
  return (
    <div className="flex items-center gap-2">
      {unsaved > 0 ? <DraftUnsavedPill count={unsaved} /> : null}
      <Button
        disabled={unsaved === 0}
        onClick={() => goToState(hub)}
        size="sm"
        type="button"
        variant="ghost"
      >
        Discard
      </Button>
      <Button disabled={unsaved === 0} onClick={() => goToState(hub)} size="sm" type="button">
        Save
      </Button>
    </div>
  );
}
