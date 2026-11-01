/** S12, S13 and S14, shown inside the hosts they change. */
import { Bell, CircleHelp, Trash2 } from "lucide-react";
import { useState } from "react";

import {
  Marker,
  PillGroup,
  Pill,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  cn,
} from "@compozy/ui";

import {
  TRIGGER_MENU_ITEMS_CLASS,
  TRIGGER_MENU_POPOVER_CLASS,
} from "@/components/assistant-ui/session-composer-trigger-menu-classes";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import { DraftShell } from "../persistent-agents-shared/draft-shell";
import {
  CODE_ENGINEER,
  GENERAL,
  ORCHESTRATOR,
  TRIAGE,
  type DraftAgent,
} from "../persistent-agents-shared/fixtures";
import { DraftComposer } from "../persistent-agents-shared/transcript";

function MenuRow({ agent, active = false }: { agent: DraftAgent; active?: boolean }) {
  return (
    <div className={cn("flex items-center gap-2.5 rounded-md px-2 py-1.5", active && "bg-hover")}>
      <AgentAvatar
        appearance={agent.appearance}
        label={agent.label}
        provider={agent.provider}
        quietIdle
        size={24}
        surfaceClassName="ring-popover"
      />
      <span className="text-small-body text-fg">{agent.label}</span>
      <span className="truncate text-eyebrow text-subtle">{agent.description}</span>
    </div>
  );
}

export function MentionComposer() {
  return (
    <div className="relative mx-auto w-full max-w-3xl">
      <div className={cn(TRIGGER_MENU_POPOVER_CLASS, "bottom-[calc(100%-0.5rem)] mx-6")}>
        <div className={TRIGGER_MENU_ITEMS_CLASS}>
          <p className="px-2 py-1 text-eyebrow text-subtle">Agents</p>
          <MenuRow active agent={ORCHESTRATOR} />
          <MenuRow agent={CODE_ENGINEER} />
          <MenuRow agent={GENERAL} />
        </div>
      </div>
      <DraftComposer placeholder="">Ask @</DraftComposer>
    </div>
  );
}

function AgentChip({ agent, unresolved = false }: { agent: DraftAgent; unresolved?: boolean }) {
  return (
    <span
      className={cn(
        "inline-flex items-baseline gap-1 align-baseline",
        unresolved ? "text-subtle line-through" : "text-fg"
      )}
    >
      <AgentAvatar
        appearance={agent.appearance}
        className="translate-y-0.5"
        label={agent.label}
        quietIdle
        size={14}
        state={unresolved ? "unknown" : "idle"}
      />
      {agent.label}
    </span>
  );
}

export function UnresolvedComposer() {
  return (
    <DraftComposer placeholder="">
      Ask <AgentChip agent={ORCHESTRATOR} /> and{" "}
      <AgentChip
        agent={{
          ...CODE_ENGINEER,
          label: "Release Captain",
          appearance: { avatar: "generated", seed: "release-captain" },
        }}
        unresolved
      />{" "}
      to compare notes.
    </DraftComposer>
  );
}

export function DeletedAgentMarker() {
  return (
    <Marker icon={<Trash2 aria-hidden="true" className="size-3" />}>
      <b>Release Captain</b> was deleted. Its sessions stay in history.
    </Marker>
  );
}

/** Stand-in for `attention-toast`, reading only the `agent_name` an old payload carries. */
export function OldNotification() {
  return (
    <div className="pointer-events-none absolute top-16 right-4 z-30 flex w-80 items-start gap-3 rounded-lg bg-popover p-3 shadow-pop">
      <AgentAvatar
        appearance={{ avatar: "generated", seed: "code_implementer" }}
        label="Code Implementer"
        quietIdle
        size={28}
        state="unknown"
        surfaceClassName="ring-popover"
      />
      <div className="flex min-w-0 flex-col gap-0.5">
        <span className="text-small-body font-medium text-fg">Code Implementer needs you</span>
        <span className="text-small-body text-muted">Migrate tokens is waiting for an answer.</span>
      </div>
      <Bell aria-hidden="true" className="ml-auto size-3.5 shrink-0 text-subtle" />
    </div>
  );
}

const ENTRIES = [
  {
    when: "23:41",
    tool: "Read internal/task/store.go",
    who: TRIAGE,
    initiator: "Automation",
    watching: false,
    decision: "Allowed",
  },
  {
    when: "23:30",
    tool: "Run make migrate",
    who: CODE_ENGINEER,
    initiator: "Daemon",
    watching: false,
    decision: "Expired",
  },
  {
    when: "22:12",
    tool: "Edit web/src/app.tsx",
    who: GENERAL,
    initiator: "You",
    watching: true,
    decision: "Allowed",
  },
  {
    when: "21:02",
    tool: "Write docs/notes.md",
    who: ORCHESTRATOR,
    initiator: "Extension spec-cycle",
    watching: false,
    decision: "Allowed",
  },
];

const LEGACY = {
  when: "Oct 12",
  tool: "Run bun test",
  who: GENERAL,
  initiator: "Unknown",
  watching: true,
  decision: "Allowed",
};

export function AuditScreen({
  initial,
  legacy = false,
}: {
  initial: "all" | "unattended";
  legacy?: boolean;
}) {
  const [filter, setFilter] = useState<string>(initial);
  const rows = [...ENTRIES, ...(legacy ? [LEGACY] : [])].filter(
    row => filter === "all" || !row.watching
  );
  return (
    <DraftShell app="dashboard" sessionBadge={0} title="Activity">
      <div className="flex flex-col gap-4 p-6">
        <PillGroup
          items={[
            { value: "all", label: "All" },
            { value: "unattended", label: "Nobody watching" },
          ]}
          onChange={setFilter}
          value={filter}
        />
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>When</TableHead>
              <TableHead>Action</TableHead>
              <TableHead>Agent</TableHead>
              <TableHead>Started by</TableHead>
              <TableHead>Decision</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map(row => (
              <TableRow key={row.tool}>
                <TableCell className="font-mono text-mono-id">{row.when}</TableCell>
                <TableCell>{row.tool}</TableCell>
                <TableCell>
                  <span className="inline-flex items-center gap-2">
                    <AgentAvatar
                      appearance={row.who.appearance}
                      label={row.who.label}
                      quietIdle
                      size={18}
                    />
                    {row.who.label}
                  </span>
                </TableCell>
                <TableCell>
                  {row.initiator === "Unknown" ? (
                    <span className="inline-flex items-center gap-1 text-subtle">
                      <CircleHelp aria-hidden="true" className="size-3" />
                      Unknown
                    </span>
                  ) : (
                    row.initiator
                  )}
                </TableCell>
                <TableCell>
                  <Pill size="sm" tone={row.decision === "Expired" ? "warning" : "neutral"}>
                    {row.decision}
                  </Pill>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </DraftShell>
  );
}
