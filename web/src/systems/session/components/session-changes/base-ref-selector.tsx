import { useQuery } from "@tanstack/react-query";
import { useDeferredValue, useState } from "react";

import {
  Button,
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  Pill,
  Popover,
  PopoverContent,
  PopoverTrigger,
  Spinner,
} from "@compozy/ui";
import { Check, ChevronDown } from "@compozy/ui/icons";

import { sessionChangeBasesOptions } from "../../lib/session-changes-query";

export interface BaseRefSelectorProps {
  workspaceId: string;
  sessionId: string;
  /** The base the summary compared against. */
  value: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSelect: (ref: string) => void;
}

/**
 * The `head → base ▾` base picker (US-008.AC-2): local and remote-tracking branches from the
 * daemon's base list, filtered on the server, with the default marked.
 */
export function BaseRefSelector({
  workspaceId,
  sessionId,
  value,
  open,
  onOpenChange,
  onSelect,
}: BaseRefSelectorProps) {
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query.trim());
  const bases = useQuery({
    ...sessionChangeBasesOptions(workspaceId, sessionId, deferredQuery),
    enabled: open,
  });
  const refs = bases.data?.refs ?? [];
  const typed = query.trim();
  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger
        render={
          <Button
            type="button"
            variant="quiet"
            size="xs"
            className="min-w-0 font-mono"
            aria-label={`Base ${value}`}
            data-testid="changes-base-selector"
          />
        }
      >
        <span className="min-w-0 truncate">{value}</span>
        <ChevronDown aria-hidden data-icon="inline-end" />
      </PopoverTrigger>
      <PopoverContent align="start" className="w-64 gap-0 p-0">
        <Command shouldFilter={false} className="bg-transparent p-0 shadow-none">
          <CommandInput
            autoFocus
            value={query}
            onValueChange={setQuery}
            placeholder="Filter branches…"
            aria-label="Filter branches"
          />
          <CommandList className="max-h-64">
            {bases.isPending && !typed ? (
              <div className="flex justify-center p-3">
                <Spinner aria-label="Loading branches" />
              </div>
            ) : (
              <CommandEmpty>
                {bases.isError ? "Couldn't list branches" : "No branches match"}
              </CommandEmpty>
            )}
            <CommandGroup>
              {refs.map(ref => (
                <CommandItem
                  key={ref.ref}
                  value={ref.ref}
                  data-testid="changes-base-option"
                  onSelect={() => {
                    onSelect(ref.ref);
                    onOpenChange(false);
                  }}
                >
                  <Check
                    aria-hidden
                    className={ref.ref === value ? "size-3.5" : "size-3.5 opacity-0"}
                  />
                  <span className="min-w-0 flex-1 truncate font-mono text-xs">{ref.ref}</span>
                  {ref.is_default ? (
                    <Pill size="xs" tone="neutral">
                      default
                    </Pill>
                  ) : null}
                </CommandItem>
              ))}
              {/* Any ref git resolves (`main~1`, a tag, a SHA) is a valid base. */}
              {typed && !refs.some(ref => ref.ref === typed) ? (
                <CommandItem
                  value={`custom:${typed}`}
                  data-testid="changes-base-custom"
                  onSelect={() => {
                    onSelect(typed);
                    onOpenChange(false);
                  }}
                >
                  <span className="min-w-0 flex-1 truncate text-xs">
                    Compare against <span className="font-mono">{typed}</span>
                  </span>
                </CommandItem>
              ) : null}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
