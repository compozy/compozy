"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import {
  ListingRowDescription,
  ListingRowIcon,
  ListingRowLink,
  ListingRowMain,
  ListingRowMeta,
  ListingRowMetaDot,
  ListingRowName,
  ListingRowSlug,
  ListingRowStat,
  ListingRowStatLabel,
  ListingRowStatValue,
  ListingRowTitle,
  ListingRowTrail,
} from "./listing-row-parts";

interface ListingRowProps extends React.ComponentProps<"div"> {
  selected?: boolean;
  interactive?: boolean;
}

function ListingRow({
  selected = false,
  interactive = true,
  className,
  ...props
}: ListingRowProps) {
  return (
    <div
      data-slot="listing-row"
      data-selected={selected ? "true" : undefined}
      data-interactive={interactive ? "true" : undefined}
      className={cn(
        "grid grid-cols-[var(--size-icon-well-row)_minmax(0,1fr)_auto] items-center gap-3.5 border-b border-line-soft px-4 py-3 text-fg transition-colors duration-base ease-out last:border-b-0",
        interactive && "hover:bg-row-hover",
        selected && "bg-row-selected",
        className
      )}
      {...props}
    />
  );
}

const ListingRowStatCompound = Object.assign(ListingRowStat, {
  Value: ListingRowStatValue,
  Label: ListingRowStatLabel,
});

const ListingRowCompound = Object.assign(ListingRow, {
  Link: ListingRowLink,
  Icon: ListingRowIcon,
  Main: ListingRowMain,
  Name: ListingRowName,
  Title: ListingRowTitle,
  Slug: ListingRowSlug,
  Description: ListingRowDescription,
  Meta: ListingRowMeta,
  MetaDot: ListingRowMetaDot,
  Trail: ListingRowTrail,
  Stat: ListingRowStatCompound,
});

export { ListingRowCompound as ListingRow };
export type {
  ListingRowDescriptionProps,
  ListingRowIconProps,
  ListingRowLinkProps,
  ListingRowMainProps,
  ListingRowMetaProps,
  ListingRowNameProps,
  ListingRowSlugProps,
  ListingRowStatProps,
  ListingRowTitleProps,
  ListingRowTrailProps,
} from "./listing-row-parts";
export type { ListingRowProps };
