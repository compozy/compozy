import { useState } from "react";

import {
  CommandItem,
  CommandSelect,
  CommandSelectShell,
  CommandSelectTrigger,
  Eyebrow,
} from "@compozy/ui";

import { AgentIcon } from "./agent-icon";
import { AgentCommandList } from "./agent-command-list";
import { formatCategoryLabel } from "../lib/agent-category";
import type { AgentPayload } from "../types";

export interface AgentCommandSelectProps {
  agents: AgentPayload[];
  value: string | null;
  onChange: (next: string | null) => void;
  placeholder?: string;
  /**
   * Renders a leading item that clears the selection. Surfaces where "no agent"
   * is a real, named configuration (a role default) pass their own wording.
   */
  clearLabel?: string;
  disabled?: boolean;
  /** The query is loading its first catalog page; existing cached agents stay selectable. */
  loading?: boolean;
  /** The catalog could not load; its message remains visible in the command empty state. */
  error?: string | null;
  triggerTestId?: string;
  triggerId?: string;
  className?: string;
}

/**
 * A configured value the catalog does not carry is shown as-is rather than
 * collapsed into the placeholder — the config really does name it, and the
 * runtime reports the mismatch separately.
 */
function UnknownAgentValue({ name }: { name: string }) {
  return (
    <span className="flex min-w-0 flex-1 items-center gap-2 text-left">
      <span className="truncate font-mono text-mono-id text-fg">{name}</span>
      <Eyebrow className="ml-auto shrink-0 text-warning">Not available</Eyebrow>
    </span>
  );
}

/** Catalog state rides beside a resolved selection: it stays usable while stale. */
function CatalogStatus({
  catalogError,
  loading,
}: {
  catalogError: string | null;
  loading: boolean;
}) {
  if (!catalogError && !loading) return null;
  return (
    <Eyebrow
      className={catalogError ? "ml-auto text-danger" : "ml-auto text-muted"}
      data-testid="agent-command-select-catalog-status"
    >
      {catalogError ? "Unavailable" : "Loading"}
    </Eyebrow>
  );
}

function SelectedAgentValue({
  agent,
  catalogError,
  loading,
}: {
  agent: AgentPayload;
  catalogError: string | null;
  loading: boolean;
}) {
  const hasCategory = Boolean(agent.category_path && agent.category_path.length > 0);
  return (
    <span className="flex min-w-0 flex-1 items-center gap-2 text-left">
      <AgentIcon provider={agent.provider} size="xs" className="shrink-0 text-muted" />
      <span className="truncate text-small-body text-fg">{agent.name}</span>
      <Eyebrow className="text-muted">{agent.provider}</Eyebrow>
      {hasCategory ? (
        <Eyebrow
          className="text-muted ml-auto truncate"
          data-testid="agent-command-select-trigger-category"
        >
          {formatCategoryLabel(agent.category_path)}
        </Eyebrow>
      ) : null}
      <CatalogStatus catalogError={catalogError} loading={loading} />
    </span>
  );
}

function EmptySelectionLabel({
  catalogError,
  loading,
  label,
}: {
  catalogError: string | null;
  loading: boolean;
  label: string;
}) {
  if (catalogError) return <span className="truncate text-danger">Unable to load agents</span>;
  return <span className="truncate text-muted">{loading ? "Loading agents…" : label}</span>;
}

interface AgentCommandSelectValueProps {
  selectedAgent: AgentPayload | null;
  selectedName: string | null;
  catalogError: string | null;
  loading: boolean;
  emptyLabel: string;
}

function AgentCommandSelectValue({
  selectedAgent,
  selectedName,
  catalogError,
  loading,
  emptyLabel,
}: AgentCommandSelectValueProps) {
  if (selectedAgent) {
    return (
      <SelectedAgentValue agent={selectedAgent} catalogError={catalogError} loading={loading} />
    );
  }
  if (selectedName) return <UnknownAgentValue name={selectedName} />;
  return <EmptySelectionLabel catalogError={catalogError} loading={loading} label={emptyLabel} />;
}

function ClearAgentItem({
  label,
  checked,
  onSelect,
}: {
  label: string;
  checked: boolean;
  onSelect: () => void;
}) {
  return (
    <CommandItem
      value={label}
      onSelect={onSelect}
      data-checked={checked ? "true" : "false"}
      data-testid="agent-command-item-clear"
    >
      <span className="truncate text-small-body text-fg">{label}</span>
    </CommandItem>
  );
}

export function AgentCommandSelect({
  agents,
  value,
  onChange,
  placeholder = "Select an agent",
  clearLabel,
  disabled,
  loading = false,
  error,
  triggerTestId,
  triggerId,
  className,
}: AgentCommandSelectProps) {
  const [open, setOpen] = useState(false);
  const selectedName = value?.trim() ? value : null;
  const selectedAgent = agents.find(agent => agent.name === selectedName) ?? null;
  const catalogError = error?.trim() || null;
  const loadingFirstPage = loading && agents.length === 0;
  const isSelected = (agent: AgentPayload) => agent.name === selectedName;
  const choose = (next: string | null) => {
    onChange(next);
    setOpen(false);
  };

  return (
    <CommandSelect open={open} onOpenChange={next => setOpen(next)}>
      <CommandSelectTrigger
        id={triggerId}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-busy={loading || undefined}
        aria-invalid={catalogError ? true : undefined}
        data-testid={triggerTestId}
        disabled={disabled || loadingFirstPage}
        className={className}
        selected={selectedAgent !== null}
      >
        <AgentCommandSelectValue
          selectedAgent={selectedAgent}
          selectedName={selectedName}
          catalogError={catalogError}
          loading={loading}
          emptyLabel={clearLabel ?? placeholder}
        />
      </CommandSelectTrigger>
      <CommandSelectShell
        className="min-w-64"
        inputPlaceholder="Search agents..."
        inputProps={{ "data-testid": "agent-command-input" }}
      >
        <AgentCommandList
          agents={agents}
          emptyState={catalogError ?? (loading ? "Loading agents…" : undefined)}
          isSelected={isSelected}
          onSelect={agent => choose(agent.name)}
          leadingItems={
            clearLabel ? (
              <ClearAgentItem
                label={clearLabel}
                checked={selectedName === null}
                onSelect={() => choose(null)}
              />
            ) : null
          }
        />
      </CommandSelectShell>
    </CommandSelect>
  );
}
