import type { ReactNode } from "react";
import { Bot, Globe, Plus, Search, Terminal } from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  Button,
  Checkbox,
  cn,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
  KindIcon,
  StatusDot,
} from "@compozy/ui";

import type { AgentMCPPickerRow } from "../lib/agent-mcp-picker-model";

export interface AgentMCPPickerDialogProps {
  agentLabel: string;
  query: string;
  onQueryChange: (query: string) => void;
  /** Every attachable server; `visibleRows` is the search-filtered subset. */
  rows: AgentMCPPickerRow[];
  visibleRows: AgentMCPPickerRow[];
  checkedCount: number;
  loading: boolean;
  error: string | null;
  onToggleServer: (name: string, checked: boolean) => void;
  onNewServer: () => void;
  onSave: () => void;
  onClose: () => void;
}

function saveLabel(count: number) {
  if (count === 0) return "Save";
  return `Save · ${count} ${count === 1 ? "server" : "servers"}`;
}

/** Searchable checklist of the person's configured servers for one agent. */
export function AgentMCPPickerDialog({
  agentLabel,
  query,
  onQueryChange,
  rows,
  visibleRows,
  checkedCount,
  loading,
  error,
  onToggleServer,
  onNewServer,
  onSave,
  onClose,
}: AgentMCPPickerDialogProps) {
  return (
    <Dialog
      open
      onOpenChange={open => {
        if (!open) onClose();
      }}
    >
      <DialogContent
        unframed
        showCloseButton={false}
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
        data-testid="agent-mcp-picker"
      >
        <EntityDialogHeader
          icon={Bot}
          title={`Tools for ${agentLabel}`}
          description="Pick the MCP servers this agent can use. It gets them on its next message."
          onClose={onClose}
        />
        <EntityDialogBody className="flex flex-col gap-3">
          {error ? (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}
          {rows.length > 0 ? (
            <InputGroup>
              <InputGroupAddon>
                <Search aria-hidden="true" />
              </InputGroupAddon>
              <InputGroupInput
                aria-label="Search servers"
                placeholder="Search your servers"
                value={query}
                onChange={event => onQueryChange(event.target.value)}
              />
            </InputGroup>
          ) : null}
          <PickerList
            rows={rows}
            visibleRows={visibleRows}
            query={query}
            loading={loading}
            hasError={Boolean(error)}
            onToggleServer={onToggleServer}
          />
        </EntityDialogBody>
        <EntityDialogFooter
          leading={
            <Button type="button" size="sm" variant="ghost" className="-ml-2" onClick={onNewServer}>
              <Plus data-icon="inline-start" />
              Add a new server
            </Button>
          }
          onCancel={onClose}
          primaryLabel={saveLabel(checkedCount)}
          onPrimary={onSave}
          primaryTestId="agent-mcp-picker-save"
        />
      </DialogContent>
    </Dialog>
  );
}

function PickerList({
  rows,
  visibleRows,
  query,
  loading,
  hasError,
  onToggleServer,
}: Pick<
  AgentMCPPickerDialogProps,
  "rows" | "visibleRows" | "query" | "loading" | "onToggleServer"
> & {
  hasError: boolean;
}) {
  if (loading) return <PlainNote>Loading your servers…</PlainNote>;
  if (rows.length === 0)
    return hasError ? null : (
      <PlainNote>
        You have no servers to pick yet. Add a new server to give it to this agent.
      </PlainNote>
    );
  if (visibleRows.length === 0)
    return <PlainNote>{`No servers match “${query.trim()}”.`}</PlainNote>;
  return (
    <ul
      className="flex flex-col overflow-hidden rounded-lg bg-sunken shadow-hairline-inset"
      data-testid="agent-mcp-picker-list"
    >
      {visibleRows.map(row => (
        <li key={row.name} className="border-b border-line-soft last:border-b-0">
          <label
            className={cn(
              "flex min-h-14 cursor-default items-center gap-3 px-3 py-2 hover:bg-surface-2/60",
              row.checked && "bg-surface-2/40"
            )}
          >
            <Checkbox
              aria-label={row.name}
              checked={row.checked}
              disabled={row.saved}
              onCheckedChange={checked => onToggleServer(row.name, checked)}
            />
            <KindIcon
              aria-hidden="true"
              icon={row.transport === "remote" ? Globe : Terminal}
              tone="well"
              size="sm"
            />
            <span className="flex min-w-0 flex-1 flex-col">
              <span className="truncate text-small-body font-medium text-fg">{row.name}</span>
              <span className="truncate text-meta text-faint">{row.source}</span>
            </span>
            <PickerStatus row={row} />
          </label>
        </li>
      ))}
    </ul>
  );
}

function PickerStatus({ row }: { row: AgentMCPPickerRow }) {
  if (row.status.kind === "needs-sign-in")
    return (
      <span className="flex shrink-0 items-center gap-1.5 text-meta text-muted-foreground">
        <StatusDot aria-hidden="true" tone="warning" size="sm" />
        Needs sign-in
      </span>
    );
  if (row.status.kind === "tools")
    return (
      <span className="shrink-0 text-meta text-faint tabular-nums">
        {row.status.count} {row.status.count === 1 ? "tool" : "tools"}
      </span>
    );
  return null;
}

function PlainNote({ children }: { children: ReactNode }) {
  return <p className="px-1 py-6 text-center text-small-body text-muted-foreground">{children}</p>;
}
