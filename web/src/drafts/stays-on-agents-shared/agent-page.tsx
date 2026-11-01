/** The agent page in the Agents app: head with crumbs, focused editors and the home all share it. */
import { Users2 } from "lucide-react";
import { useState, type ReactNode } from "react";

import {
  Button,
  ConfirmDialog,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  PageContent,
  Pill,
  TopbarOverflowIcon,
  type TopbarSlotValue,
} from "@compozy/ui";

import { goToAgents, goToProfile } from "../agent-profile/profile-nav";
import { DraftShell, DraftUnsavedPill } from "./draft-shell";
import type { DraftAgent } from "./fixtures";

export function KindPill({ agent }: { agent: Pick<DraftAgent, "kind"> }) {
  return (
    <Pill size="sm" tone="neutral">
      {agent.kind === "stays_on" ? "Stays on" : "On demand"}
    </Pill>
  );
}

export function AgentOverflow({ agent }: { agent: DraftAgent }) {
  const [deleting, setDeleting] = useState(false);
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          aria-label="More agent actions"
          render={<Button size="icon-sm" type="button" variant="quiet" />}
        >
          <TopbarOverflowIcon aria-hidden="true" className="size-4" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem>Duplicate</DropdownMenuItem>
          <DropdownMenuItem onClick={() => setDeleting(true)} variant="destructive">
            Delete…
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <ConfirmDialog
        cancelLabel="Keep"
        confirmLabel="Delete"
        description={
          agent.kind === "stays_on"
            ? "Its sessions stay in history. What it remembers is put away, not erased."
            : "Its sessions stay in history."
        }
        onConfirm={() => setDeleting(false)}
        onOpenChange={setDeleting}
        open={deleting}
        title={`Delete ${agent.label}?`}
        tone="danger"
      />
    </>
  );
}

export interface AgentAppPageProps {
  agent: DraftAgent;
  /** The focused editor's name; omitted on the home. */
  leaf?: string;
  hubState?: string;
  actions?: ReactNode;
  children: ReactNode;
  width?: string;
  sessionBadge?: number;
}

/** One location of the Agents window: `Agents / Orchestrator / Check-ins`. */
export function AgentAppPage({
  agent,
  leaf,
  hubState = "stays-on",
  actions,
  children,
  width = "max-w-3xl",
  sessionBadge,
}: AgentAppPageProps) {
  const home = hubState === "extension" ? hubState : "stays-on";
  const crumbs = [
    { id: "agents", label: "Agents", onSelect: goToAgents },
    ...(leaf ? [{ id: agent.name, label: agent.label, onSelect: () => goToProfile(home) }] : []),
  ];
  const topbar: TopbarSlotValue = {
    crumbs,
    crumb: leaf ?? agent.label,
    onBack: () => (leaf ? goToProfile(home) : goToAgents()),
    actions,
    overflow: leaf ? undefined : <AgentOverflow agent={agent} />,
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
export function SaveActions({ unsaved, hub = "stays-on" }: { unsaved: number; hub?: string }) {
  return (
    <div className="flex items-center gap-2">
      {unsaved > 0 ? <DraftUnsavedPill count={unsaved} /> : null}
      <Button
        disabled={unsaved === 0}
        onClick={() => goToProfile(hub)}
        size="sm"
        type="button"
        variant="ghost"
      >
        Discard
      </Button>
      <Button disabled={unsaved === 0} onClick={() => goToProfile(hub)} size="sm" type="button">
        Save
      </Button>
    </div>
  );
}
