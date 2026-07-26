import { useState } from "react";

import { CommandSelect, CommandSelectShell, CommandSelectTrigger, Pill } from "@agh/ui";

import { AgentCommandList } from "./agent-command-list";
import type { AgentPayload } from "../types";

export interface AgentCommandMultiSelectProps {
  agents: AgentPayload[];
  value: string[];
  onToggle: (next: string[]) => void;
  placeholder?: string;
  disabled?: boolean;
  triggerTestId?: string;
  triggerId?: string;
  itemTestId?: (agent: AgentPayload) => string;
  className?: string;
  countTestId?: string;
}

export function AgentCommandMultiSelect({
  agents,
  value,
  onToggle,
  placeholder = "Select agents",
  disabled,
  triggerTestId,
  triggerId,
  itemTestId,
  className,
  countTestId,
}: AgentCommandMultiSelectProps) {
  const [open, setOpen] = useState(false);
  const selectedSet = new Set(value);
  const isSelected = (agent: AgentPayload) => selectedSet.has(agent.name);

  const handleSelect = (agent: AgentPayload) => {
    if (selectedSet.has(agent.name)) {
      onToggle(value.filter(name => name !== agent.name));
    } else {
      onToggle([...value, agent.name]);
    }
  };

  return (
    <CommandSelect open={open} onOpenChange={next => setOpen(next)}>
      <CommandSelectTrigger
        id={triggerId}
        aria-haspopup="listbox"
        aria-expanded={open}
        data-testid={triggerTestId}
        disabled={disabled}
        className={className}
        selected={value.length > 0}
      >
        <span className="flex min-w-0 flex-1 items-center gap-2 text-left">
          {value.length === 0 ? (
            <span className="truncate text-muted">{placeholder}</span>
          ) : (
            <span className="truncate text-sm text-fg">{value.length} selected</span>
          )}
        </span>
        <Pill mono data-testid={countTestId}>
          {value.length}
        </Pill>
      </CommandSelectTrigger>
      <CommandSelectShell
        className="min-w-64"
        inputPlaceholder="Search agents..."
        inputProps={{ "data-testid": "agent-command-input" }}
      >
        <AgentCommandList
          agents={agents}
          isSelected={isSelected}
          onSelect={handleSelect}
          itemTestId={itemTestId}
        />
      </CommandSelectShell>
    </CommandSelect>
  );
}
