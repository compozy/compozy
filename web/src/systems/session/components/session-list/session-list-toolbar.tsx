import { ArrowDownUp, Check } from "lucide-react";

import { Button, Icon, PillGroup, Popover, PopoverContent, PopoverTrigger } from "@compozy/ui";

import type { SessionListScope, SessionListSort } from "../../lib/session-list-preferences";

const SCOPE_LABELS: Record<SessionListScope, string> = {
  recent: "Recent",
  all: "All",
  "all-workspaces": "All workspaces",
};

const SORT_LABELS: Record<SessionListSort, string> = {
  last_activity: "Last activity",
  attention: "Attention first",
};

export interface SessionListToolbarProps {
  scope: SessionListScope;
  sort: SessionListSort;
  disabled?: boolean;
  onScopeChange: (scope: SessionListScope) => void;
  onSortChange: (sort: SessionListSort) => void;
  testIdPrefix: string;
}

/**
 * Scope and order for a session list. Both persist per operator through the
 * daemon, so the choice made here is the same choice the CLI reports.
 *
 * Attention-first is an option, not a takeover: last activity stays the default
 * because a list that reorders itself around urgency is harder to navigate when
 * nothing is urgent.
 */
export function SessionListToolbar({
  scope,
  sort,
  disabled = false,
  onScopeChange,
  onSortChange,
  testIdPrefix,
}: SessionListToolbarProps) {
  return (
    <div className="flex items-center gap-2 px-3 py-1.5">
      <PillGroup
        aria-label="Session scope"
        size="sm"
        className="min-w-0"
        value={scope}
        onChange={onScopeChange}
        items={(Object.keys(SCOPE_LABELS) as SessionListScope[]).map(value => ({
          value,
          label: SCOPE_LABELS[value],
          disabled,
          testId: `${testIdPrefix}-scope-${value}`,
        }))}
      />
      <Popover>
        <PopoverTrigger
          render={
            <Button
              variant="ghost"
              size="sm"
              className="ml-auto shrink-0"
              disabled={disabled}
              data-testid={`${testIdPrefix}-sort-trigger`}
            >
              <Icon as={ArrowDownUp} size="sm" />
              {SORT_LABELS[sort]}
            </Button>
          }
        />
        <PopoverContent align="end" className="w-52 p-1" aria-label="Sort sessions">
          {(Object.keys(SORT_LABELS) as SessionListSort[]).map(value => (
            <button
              key={value}
              type="button"
              role="menuitemradio"
              aria-checked={sort === value}
              disabled={disabled}
              data-testid={`${testIdPrefix}-sort-${value}`}
              className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-small-body text-fg transition-colors hover:bg-row-hover focus-visible:shadow-focus-ring focus-visible:outline-none"
              onClick={() => onSortChange(value)}
            >
              <span className="flex-1">{SORT_LABELS[value]}</span>
              {sort === value ? <Icon as={Check} size="sm" className="text-accent" /> : null}
            </button>
          ))}
        </PopoverContent>
      </Popover>
    </div>
  );
}
