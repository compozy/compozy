/** The agent page in the Agents app: head with crumbs, focused editors and the home all share it. */
import { Users2 } from "@compozy/ui/icons";
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
import { DraftShell, DraftUnsavedPill, type DraftShellProps } from "./draft-shell";
import type { DraftAgent } from "./fixtures";

export function KindPill({ agent }: { agent: Pick<DraftAgent, "kind"> }) {
  return (
    <Pill size="sm" tone="neutral">
      {agent.kind === "always_on" ? "Always-on" : "On-demand"}
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
          agent.kind === "always_on"
            ? "Its sessions stay in history. Its memory is put away, not erased."
            : "Its sessions stay in history."
        }
        onConfirm={() => setDeleting(false)}
        onOpenChange={setDeleting}
        open={deleting}
        title={`Delete ${agent.label}?`}
        tone="destructive"
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
  /** System bar overrides: the bell count and its open popover. */
  shell?: Pick<DraftShellProps, "notifications" | "wrapBellTrigger">;
}

/** One location of the Agents window: `Agents / Orchestrator / Heartbeat`. */
export function AgentAppPage({
  agent,
  leaf,
  hubState = "always-on",
  actions,
  children,
  width = "max-w-3xl",
  sessionBadge,
  shell,
}: AgentAppPageProps) {
  const home = hubState === "extension" ? hubState : "always-on";
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
      {...shell}
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
export function SaveActions({ unsaved, hub = "always-on" }: { unsaved: number; hub?: string }) {
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
