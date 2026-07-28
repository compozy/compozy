"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import {
  ListingToolbarFilters,
  ListingToolbarLeading,
  ListingToolbarSearch,
  ListingToolbarTrailing,
  ListingToolbarViewToggle,
} from "./listing-toolbar-parts";

type ListingToolbarRootProps = React.ComponentProps<"div">;

function ListingToolbar({ className, ...props }: ListingToolbarRootProps) {
  return (
    <div
      data-slot="listing-toolbar"
      data-testid="listing-toolbar"
      className={cn("flex flex-wrap items-center gap-2.5", className)}
      {...props}
    />
  );
}

const ListingToolbarCompound = Object.assign(ListingToolbar, {
  Leading: ListingToolbarLeading,
  Trailing: ListingToolbarTrailing,
  Search: ListingToolbarSearch,
  Filters: ListingToolbarFilters,
  ViewToggle: ListingToolbarViewToggle,
});

export { ListingToolbarCompound as ListingToolbar };
export type {
  ListingToolbarFiltersProps,
  ListingToolbarLeadingProps,
  ListingToolbarSearchProps,
  ListingToolbarTrailingProps,
  ListingToolbarViewToggleProps,
  ListingViewMode,
} from "./listing-toolbar-parts";
export type { ListingToolbarRootProps as ListingToolbarProps };
