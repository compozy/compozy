import { LayoutGrid, List, SearchIcon } from "lucide-react";
import * as React from "react";

import { cn } from "../../lib/utils";
import { Button } from "../button";
import { useCollapsibleSearch } from "./hooks/use-collapsible-search";
import { PillGroup } from "./pill-group";
import { SearchInput, type SearchInputProps } from "./search-input";

export type ListingViewMode = "rows" | "cards";
export type ListingToolbarLeadingProps = React.ComponentProps<"div">;
export type ListingToolbarTrailingProps = React.ComponentProps<"div">;
export type ListingToolbarFiltersProps = React.ComponentProps<"div">;
export interface ListingToolbarSearchProps extends SearchInputProps {
  /**
   * The strip is too narrow for a usable field. The search shows as an icon
   * button that opens the field on demand and folds back once the field is
   * empty and loses focus (or Escape is pressed); a field holding a query
   * stays open, since a clipped field is worse than none but a hidden query
   * is worse still.
   */
  collapsed?: boolean;
}

export interface ListingToolbarViewToggleProps extends Omit<
  React.ComponentProps<"div">,
  "onChange"
> {
  value: ListingViewMode;
  onChange: (next: ListingViewMode) => void;
}

const VIEW_ITEMS = [
  {
    value: "rows" as const,
    label: (
      <span className="inline-flex items-center gap-1.5">
        <List aria-hidden="true" />
        Rows
      </span>
    ),
    testId: "listing-view-rows",
  },
  {
    value: "cards" as const,
    label: (
      <span className="inline-flex items-center gap-1.5">
        <LayoutGrid aria-hidden="true" />
        Cards
      </span>
    ),
    testId: "listing-view-cards",
  },
];

/**
 * Search and filters on one line. Toolbars sit in a fixed-height window strip,
 * so nothing wraps: in a narrow pane the search gives way down to its floor,
 * then the strip scrolls sideways instead of stacking controls onto each other.
 */
export function ListingToolbarLeading({ className, ...props }: ListingToolbarLeadingProps) {
  return (
    <div
      data-slot="listing-toolbar-leading"
      className={cn("flex flex-1 flex-nowrap items-center gap-2.5", className)}
      {...props}
    />
  );
}

export function ListingToolbarTrailing({ className, ...props }: ListingToolbarTrailingProps) {
  return (
    <div
      data-slot="listing-toolbar-trailing"
      className={cn("ml-auto flex shrink-0 items-center", className)}
      {...props}
    />
  );
}

export function ListingToolbarSearch({
  kbd = "/",
  containerClassName,
  collapsed = false,
  ref,
  onBlur,
  onKeyDown,
  ...props
}: ListingToolbarSearchProps) {
  const hasQuery = (props.value ?? String(props.defaultValue ?? "")) !== "";
  const { showToggle, open, close, inputRefs, toggleRef } = useCollapsibleSearch(
    collapsed,
    hasQuery,
    ref
  );

  if (showToggle) {
    return (
      <Button
        aria-label={props["aria-label"] ?? props.placeholder ?? "Search"}
        data-slot="listing-toolbar-search-toggle"
        onClick={open}
        ref={toggleRef}
        size="segment"
        type="button"
        variant="quiet"
      >
        <SearchIcon aria-hidden="true" className="size-3 text-subtle" />
      </Button>
    );
  }
  return (
    <SearchInput
      kbd={kbd}
      data-testid="listing-search-input"
      containerClassName={cn(
        "basis-search-input min-w-search-input-floor shrink",
        containerClassName
      )}
      ref={inputRefs}
      onBlur={event => {
        onBlur?.(event);
        if (event.currentTarget.value === "") close(false);
      }}
      onKeyDown={event => {
        onKeyDown?.(event);
        if (collapsed && event.key === "Escape" && event.currentTarget.value === "") {
          // Consumed: a host's document-level Escape (e.g. the desktop's
          // focus return) must not pull focus off the toggle.
          event.preventDefault();
          close(true);
        }
      }}
      {...props}
    />
  );
}

export function ListingToolbarFilters({ className, ...props }: ListingToolbarFiltersProps) {
  return (
    <div
      data-slot="listing-toolbar-filters"
      className={cn("flex shrink-0 flex-nowrap items-center", className)}
      {...props}
    />
  );
}

export function ListingToolbarViewToggle({
  value,
  onChange,
  className,
  ...props
}: ListingToolbarViewToggleProps) {
  return (
    <div data-slot="listing-toolbar-view" className={cn(className)} {...props}>
      <PillGroup<ListingViewMode>
        aria-label="View mode"
        data-testid="listing-view-toggle"
        items={VIEW_ITEMS}
        onChange={onChange}
        size="md"
        value={value}
      />
    </div>
  );
}
