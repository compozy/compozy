import { useState } from "react";

import {
  CommandItem,
  CommandSelect,
  CommandSelectShell,
  CommandSelectTrigger,
  Eyebrow,
} from "@agh/ui";

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
      <span className="truncate font-mono text-sm text-fg">{name}</span>
      <Eyebrow className="ml-auto shrink-0 text-warning">Not available</Eyebrow>
    </span>
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
  const hasCatalogItems = agents.length > 0;
  const catalogStatus = catalogError ? "Unavailable" : loading ? "Loading" : null;
  const isSelected = (agent: AgentPayload) => agent.name === selectedName;
  const handleSelect = (agent: AgentPayload) => {
    onChange(agent.name);
    setOpen(false);
  };
  const handleClear = () => {
    onChange(null);
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
        disabled={disabled || (loading && !hasCatalogItems)}
        className={className}
        selected={Boolean(selectedAgent)}
      >
        {selectedAgent ? (
          <span className="flex min-w-0 flex-1 items-center gap-2 text-left">
            <AgentIcon
              provider={selectedAgent.provider}
              size="xs"
              className="shrink-0 text-muted"
            />
            <span className="truncate text-sm text-fg">{selectedAgent.name}</span>
            <Eyebrow className="text-muted">{selectedAgent.provider}</Eyebrow>
            {selectedAgent.category_path && selectedAgent.category_path.length > 0 ? (
              <Eyebrow
                className="text-muted ml-auto truncate"
                data-testid="agent-command-select-trigger-category"
              >
                {formatCategoryLabel(selectedAgent.category_path)}
              </Eyebrow>
            ) : null}
            {catalogStatus ? (
              <Eyebrow
                className={catalogError ? "ml-auto text-danger" : "ml-auto text-muted"}
                data-testid="agent-command-select-catalog-status"
              >
                {catalogStatus}
              </Eyebrow>
            ) : null}
          </span>
        ) : selectedName ? (
          <UnknownAgentValue name={selectedName} />
        ) : (
          <span className={catalogError ? "truncate text-danger" : "truncate text-muted"}>
            {catalogError
              ? "Unable to load agents"
              : loading
                ? "Loading agents…"
                : (clearLabel ?? placeholder)}
          </span>
        )}
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
          onSelect={handleSelect}
          leadingItems={
            clearLabel ? (
              <CommandItem
                value={clearLabel}
                onSelect={handleClear}
                data-checked={selectedName ? "false" : "true"}
                data-testid="agent-command-item-clear"
              >
                <span className="truncate text-sm text-fg">{clearLabel}</span>
              </CommandItem>
            ) : null
          }
        />
      </CommandSelectShell>
    </CommandSelect>
  );
}
