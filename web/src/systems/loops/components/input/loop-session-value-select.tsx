import { useState } from "react";
import {
  Button,
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
} from "@compozy/ui";

import { getSessionDisplayTitle } from "@/systems/session";

import { useLoopSessionCatalog } from "../../hooks/use-loop-session-catalog";

interface LoopSessionValueSelectProps {
  workspaceId: string;
  value: string;
  controlId: string;
  testId: string;
  disabled?: boolean;
  invalid?: boolean;
  describedBy?: string;
  onChange: (value: string) => void;
}

export function LoopSessionValueSelect({
  workspaceId,
  value,
  controlId,
  testId,
  disabled,
  invalid,
  describedBy,
  onChange,
}: LoopSessionValueSelectProps) {
  const [open, setOpen] = useState(false);
  const catalog = useLoopSessionCatalog(workspaceId, value);
  return (
    <CommandSelect open={open} onOpenChange={setOpen}>
      <CommandSelectTrigger
        id={controlId}
        data-testid={testId}
        aria-describedby={describedBy}
        aria-invalid={invalid || undefined}
        aria-busy={catalog.loading || catalog.selectedLoading || undefined}
        disabled={disabled}
        selected={value !== ""}
      >
        <span className="min-w-0 flex-1 truncate text-left text-small-body">
          {catalog.selected ? getSessionDisplayTitle(catalog.selected) : value || "Select session"}
        </span>
      </CommandSelectTrigger>
      <CommandSelectShell
        commandProps={{ shouldFilter: false, label: "Search sessions" }}
        inputPlaceholder="Search session titles or agents..."
        inputProps={{
          value: catalog.search,
          onValueChange: catalog.setSearch,
          "aria-label": "Search sessions",
        }}
      >
        <CommandList>
          <CommandEmpty>
            {catalog.loading
              ? "Loading sessions…"
              : catalog.failed
                ? "Sessions could not be loaded."
                : "No sessions match your search."}
          </CommandEmpty>
          <CommandSelectGroup>
            {catalog.sessions.map(session => (
              <CommandItem
                key={session.id}
                value={session.id}
                data-checked={session.id === value ? "true" : "false"}
                onSelect={() => {
                  onChange(session.id);
                  setOpen(false);
                }}
              >
                <span className="min-w-0 flex-1 truncate text-small-body">
                  {getSessionDisplayTitle(session)}
                </span>
                <span className="truncate font-mono text-mono-id text-subtle">
                  {session.agent_name}
                </span>
              </CommandItem>
            ))}
          </CommandSelectGroup>
        </CommandList>
        {catalog.failed ? (
          <div role="status" className="px-2 py-1 text-form-hint text-warning">
            Unable to load sessions.{" "}
            <Button type="button" size="sm" variant="ghost" onClick={catalog.retry}>
              Retry
            </Button>
          </div>
        ) : null}
        <div className="flex justify-between gap-2 border-t border-line p-2">
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={!catalog.previous || catalog.paging || catalog.loading}
            onClick={catalog.previousPage}
          >
            Previous
          </Button>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={!catalog.next || catalog.paging || catalog.loading}
            onClick={catalog.nextPage}
          >
            Next
          </Button>
        </div>
      </CommandSelectShell>
    </CommandSelect>
  );
}
