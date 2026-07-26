import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
  ListingToolbar,
} from "@agh/ui";

import type { ProviderStateLabel } from "../lib/provider-state";

export type ProvidersViewMode = "rows" | "cards";

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
  view: ProvidersViewMode;
  onViewChange: (next: ProvidersViewMode) => void;
}

export function ProvidersToolbar({
  nameQuery,
  onNameQueryChange,
  statusFilter,
  onStatusChange,
  view,
  onViewChange,
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
              className="inline-flex h-7 items-center gap-1 rounded-md border border-line bg-btn-default-fill px-2.5 text-form-label text-fg transition-colors duration-base hover:bg-btn-default-hover focus-visible:outline-none focus-visible:shadow-focus-ring"
              data-testid="settings-providers-status-filter"
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
      <ListingToolbar.Trailing>
        <ListingToolbar.ViewToggle onChange={onViewChange} value={view} />
      </ListingToolbar.Trailing>
    </ListingToolbar>
  );
}
