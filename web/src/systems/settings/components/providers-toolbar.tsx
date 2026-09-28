import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
  ListingToolbar,
} from "@compozy/ui";

import type { ProviderStateLabel } from "../lib/provider-state";

const STATUS_LABEL: Record<ProviderStateLabel | "all", string> = {
  all: "All",
  installed: "Ready",
  unconfigured: "Needs setup",
  "binary-missing": "Not installed",
  "needs-sign-in": "Needs sign-in",
  "auth-unknown": "Sign-in unverified",
  "auth-unavailable": "Sign-in unavailable",
};

export interface ProvidersToolbarProps {
  nameQuery: string;
  onNameQueryChange: (next: string) => void;
  statusFilter: ProviderStateLabel | null;
  onStatusChange: (next: ProviderStateLabel | null) => void;
}

export function ProvidersToolbar({
  nameQuery,
  onNameQueryChange,
  statusFilter,
  onStatusChange,
}: ProvidersToolbarProps) {
  const statusValue = statusFilter ?? "all";

  return (
    <ListingToolbar data-testid="settings-providers-toolbar">
      <ListingToolbar.Leading>
        <ListingToolbar.Search
          aria-label="Search providers"
          data-testid="settings-providers-search"
          onChange={onNameQueryChange}
          placeholder="Search providers"
          value={nameQuery}
        />
        <ListingToolbar.Filters>
          <DropdownMenu>
            <DropdownMenuTrigger
              aria-label="Filter by status"
              data-testid="settings-providers-status-filter"
              render={<Button size="sm" type="button" variant="outline" />}
            >
              <span className="text-subtle">Status</span>
              <span aria-hidden="true" className="text-faint">
                :
              </span>
              <span className="font-medium">{STATUS_LABEL[statusValue]}</span>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              <DropdownMenuRadioGroup
                onValueChange={value =>
                  onStatusChange(value === "all" ? null : (value as ProviderStateLabel))
                }
                value={statusValue}
              >
                {(Object.keys(STATUS_LABEL) as Array<ProviderStateLabel | "all">).map(value => (
                  <DropdownMenuRadioItem
                    data-testid={`settings-providers-status-${value}`}
                    key={value}
                    value={value}
                  >
                    {STATUS_LABEL[value]}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        </ListingToolbar.Filters>
      </ListingToolbar.Leading>
    </ListingToolbar>
  );
}
