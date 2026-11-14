/**
 * The CompozyOS group (`_uiux.md` S1): today's `MCPServersTable` anatomy, plus the approval
 * cell for project-file servers that wait for the operator's "Allow". The status pill for every
 * other row comes from the production view model (`composeMCPRowStatus`).
 */
import { Pencil, Plug } from "@compozy/ui/icons";

import { Button, cn, Pill } from "@compozy/ui";

import { mcpServerProvenanceLine } from "@/systems/settings/components/mcp-server-labels";
import { mcpDefinitionKey } from "@/systems/settings/lib/mcp-management-target";
import { composeMCPRowStatus } from "@/systems/settings/lib/mcp-status-view-model";
import type { SettingsMCPServerEntry } from "@/systems/settings/types";

export type ApprovalOverlay = Record<string, "waiting" | "changed">;

const ROW_GRID = "md:grid-cols-[minmax(0,1fr)_auto_auto]";
const ROW_CLASS = cn(
  "grid grid-cols-2 gap-x-3.5 gap-y-3 border-t border-line-soft p-3.5 transition-colors first:border-t-0",
  ROW_GRID,
  "md:min-h-setting-row md:items-center md:gap-y-0"
);

function GroupHeading({ count, waiting }: { count: number; waiting: number }) {
  return (
    <header className="flex min-h-12 items-center gap-3 border-b border-line px-4 py-2.5">
      <span
        aria-hidden="true"
        className="grid size-7 place-items-center rounded-md bg-brand-tint text-brand-strong"
      >
        <Plug className="size-4" />
      </span>
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="flex items-center gap-2">
          <span className="text-small-body font-medium text-fg">CompozyOS</span>
          <Pill mono>{count}</Pill>
        </span>
        <span className="truncate text-eyebrow text-subtle">
          {waiting > 0
            ? `Every CompozyOS session can use these · ${waiting} waiting for approval`
            : "Every CompozyOS session can use these from its next message"}
        </span>
      </div>
    </header>
  );
}

function ApprovalCell({ state }: { state: "waiting" | "changed" }) {
  return (
    <div className="flex min-w-0 flex-col gap-0.5">
      <Pill form="plain" tone="warning">
        <Pill.Dot />
        Waiting for approval
      </Pill>
      <span className="text-eyebrow text-subtle">
        {state === "changed" ? "Changed since you allowed it" : "Added by a file in this project"}
      </span>
    </div>
  );
}

function CompozyRow({
  server,
  approval,
  highlighted,
  onAllow,
}: {
  server: SettingsMCPServerEntry;
  approval?: "waiting" | "changed";
  highlighted: boolean;
  onAllow: (name: string) => void;
}) {
  const status = composeMCPRowStatus(server);
  return (
    <div className={cn(ROW_CLASS, highlighted ? "bg-selected" : "hover:bg-surface-2")}>
      <div className="col-span-2 flex min-w-0 items-center gap-2.5 md:col-span-1">
        <span
          aria-hidden="true"
          className="grid size-8 shrink-0 place-items-center rounded-md bg-surface-2 text-muted-foreground"
        >
          <Plug className="size-4" />
        </span>
        <div className="min-w-0">
          <span className="truncate font-mono text-small-body font-medium text-fg">
            {server.name}
          </span>
          <div className="mt-0.5 truncate text-eyebrow text-subtle">
            {mcpServerProvenanceLine(server)}
          </div>
        </div>
      </div>
      <div className="min-w-0">
        {approval ? (
          <ApprovalCell state={approval} />
        ) : (
          <Pill form="plain" tone={status.summary.tone}>
            <Pill.Dot />
            {status.summary.label}
          </Pill>
        )}
      </div>
      <div className="flex items-center justify-end gap-1.5">
        {approval ? (
          <Button onClick={() => onAllow(server.name)} size="sm" type="button" variant="neutral">
            Allow
          </Button>
        ) : status.repairable && status.authorizeLabel ? (
          <Button size="sm" type="button" variant="neutral">
            {status.authorizeLabel}
          </Button>
        ) : null}
        <Button aria-label={`Edit ${server.name}`} size="icon-sm" type="button" variant="ghost">
          <Pencil className="size-3.5" />
        </Button>
      </div>
    </div>
  );
}

export interface CompozyGroupProps {
  servers: readonly SettingsMCPServerEntry[];
  approvals: ApprovalOverlay;
  highlighted?: string;
  onAllow: (name: string) => void;
}

export function CompozyGroup({ servers, approvals, highlighted, onAllow }: CompozyGroupProps) {
  const waiting = servers.filter(server => approvals[server.name]).length;
  return (
    <section
      aria-label="CompozyOS MCP servers"
      className="overflow-hidden rounded-lg bg-card shadow-card"
    >
      <GroupHeading count={servers.length} waiting={waiting} />
      <div>
        {servers.map(server => (
          <CompozyRow
            approval={approvals[server.name]}
            highlighted={server.name === highlighted}
            key={mcpDefinitionKey(server)}
            onAllow={onAllow}
            server={server}
          />
        ))}
      </div>
    </section>
  );
}
