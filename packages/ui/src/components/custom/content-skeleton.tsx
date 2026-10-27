import * as React from "react";

import { cycleByPosition, skeletonSlots } from "../../lib/skeleton-slots";
import { cn } from "../../lib/utils";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemMedia,
  ItemTitle,
  type ItemProps,
} from "../item";
import { Skeleton, SkeletonText } from "../skeleton";
import { TableCell, TableRow } from "../table";
import { CatalogCard } from "./catalog-card";
import { ListingRow } from "./listing-row";
import { PropertyRow, type PropertyRowVariant } from "./property-row";

// Content-shaped skeletons. Each one renders the real primitive's container and slot components
// and fills only the leaves with placeholder text and blocks, so row heights, paddings, gaps,
// icon wells and line boxes come from the real classes and cannot drift when those change.
// Every shape is presentational: it is hidden from assistive technology and non-interactive, and
// the loading semantics belong to the single `SkeletonRegion` that wraps the whole area.
// Repeated rows vary their widths deterministically so a stack does not read as a stamped block.

const TITLE_WIDTHS = ["w-40", "w-32", "w-48", "w-36"] as const;
const DESCRIPTION_WIDTHS = ["w-3/4", "w-2/3", "w-4/5", "w-3/5"] as const;
const META_WIDTHS = [
  ["w-16", "w-24"],
  ["w-20", "w-14"],
  ["w-14", "w-20"],
] as const;
const CELL_WIDTHS = ["w-32", "w-20", "w-24", "w-16"] as const;
const LABEL_WIDTHS = ["w-16", "w-12", "w-20"] as const;
const VALUE_WIDTHS = ["w-24", "w-32", "w-20"] as const;

export type ListingRowSkeletonTrail = "none" | "pill" | "stat";

export interface ListingRowSkeletonProps extends Omit<React.ComponentProps<"div">, "children"> {
  /** Rows to render. Defaults to 3. */
  count?: number;
  /** Adds the description line under the title (`ListingRow.Description`). */
  description?: boolean;
  /** Adds the meta line (`ListingRow.Meta`). On by default, as on most real rows. */
  meta?: boolean;
  /** The trailing column: a status pill, a value and label stat, or nothing. Defaults to "stat". */
  trail?: ListingRowSkeletonTrail;
}

/**
 * `count` non-interactive `ListingRow`s in skeleton form: the real row grid, icon well, name,
 * description, meta and trail slots, with placeholders where the content goes. Siblings, so put
 * them where the real rows go (a `ListGroup.Items`, a bordered list) and wrap the area in a
 * `SkeletonRegion`. Match the toggles to the real row: a row with a description line is taller.
 */
function ListingRowSkeleton({
  count = 3,
  description = false,
  meta = true,
  trail = "stat",
  className,
  ...props
}: ListingRowSkeletonProps) {
  return (
    <>
      {skeletonSlots("listing-row-skeleton", count).map(slot => (
        <ListingRow
          key={slot.id}
          aria-hidden="true"
          interactive={false}
          className={cn("animate-skeleton-reveal", className)}
          {...props}
        >
          <ListingRow.Icon>
            <Skeleton className="size-full" />
          </ListingRow.Icon>
          <ListingRow.Main>
            <ListingRow.Name>
              <ListingRow.Title>
                <SkeletonText className={cycleByPosition(TITLE_WIDTHS, slot.position)} />
              </ListingRow.Title>
            </ListingRow.Name>
            {description ? (
              <ListingRow.Description>
                <SkeletonText className={cycleByPosition(DESCRIPTION_WIDTHS, slot.position)} />
              </ListingRow.Description>
            ) : null}
            {meta ? (
              <ListingRow.Meta>
                {cycleByPosition(META_WIDTHS, slot.position).map(width => (
                  <SkeletonText key={width} className={width} />
                ))}
              </ListingRow.Meta>
            ) : null}
          </ListingRow.Main>
          {trail === "none" ? null : (
            <ListingRow.Trail>
              {trail === "pill" ? (
                <Skeleton className="h-pill-sm w-16 rounded-pill" />
              ) : (
                <ListingRow.Stat>
                  <ListingRow.Stat.Value>
                    <SkeletonText className="w-8" />
                  </ListingRow.Stat.Value>
                  <ListingRow.Stat.Label>
                    <SkeletonText className="w-14" />
                  </ListingRow.Stat.Label>
                </ListingRow.Stat>
              )}
            </ListingRow.Trail>
          )}
        </ListingRow>
      ))}
    </>
  );
}

export type ItemSkeletonMedia = "none" | "icon" | "image";

export interface ItemSkeletonProps extends Omit<React.ComponentProps<"div">, "children"> {
  /** Items to render. Defaults to 3. */
  count?: number;
  /** The `Item` size the real rail, picker or inspector row uses. */
  size?: ItemProps["size"];
  /** The `Item` variant the real row uses. */
  variant?: ItemProps["variant"];
  /** The leading media: a glyph, a sized image, or none. Defaults to "icon". */
  media?: ItemSkeletonMedia;
  /** Adds the description line under the title. On by default. */
  description?: boolean;
  /** Adds a trailing action control. */
  actions?: boolean;
}

/**
 * `count` non-interactive `Item`s in skeleton form, for the compact rails, pickers and
 * inspectors `Item` serves: the real item container and its media, content, title, description
 * and actions slots. Pass the same `size` and `variant` as the real rows. Siblings, so place them
 * in the real list container and wrap the area in a `SkeletonRegion`. `ItemGroup` wraps each of
 * its children in its own list item, so inside one render a single `<ItemSkeleton count={1} />`
 * per row instead of one with a `count`.
 */
function ItemSkeleton({
  count = 3,
  size,
  variant,
  media = "icon",
  description = true,
  actions = false,
  className,
  ...props
}: ItemSkeletonProps) {
  return (
    <>
      {skeletonSlots("item-skeleton", count).map(slot => (
        <Item
          key={slot.id}
          aria-hidden="true"
          size={size}
          variant={variant}
          className={cn("animate-skeleton-reveal", className)}
          {...props}
        >
          {media === "none" ? null : (
            <ItemMedia variant={media}>
              <Skeleton className={media === "icon" ? "size-4" : "size-full"} />
            </ItemMedia>
          )}
          <ItemContent>
            <ItemTitle>
              <SkeletonText className={cycleByPosition(TITLE_WIDTHS, slot.position)} />
            </ItemTitle>
            {description ? (
              <ItemDescription>
                <SkeletonText className={cycleByPosition(DESCRIPTION_WIDTHS, slot.position)} />
              </ItemDescription>
            ) : null}
          </ItemContent>
          {actions ? (
            <ItemActions>
              <Skeleton className="h-button-sm w-14 rounded-pill" />
            </ItemActions>
          ) : null}
        </Item>
      ))}
    </>
  );
}

export interface CatalogCardSkeletonProps extends React.ComponentProps<"article"> {
  /** Cards to render. Defaults to 1. */
  count?: number;
  /** Description lines under the header; 0 omits the description. Defaults to 2. */
  descriptionLines?: number;
  /** Adds the quiet one-line facts row under the description. On by default. */
  facts?: boolean;
  /** Adds the footer action row (`CatalogCard.Actions`). On by default. */
  actions?: boolean;
}

/**
 * `count` non-interactive `CatalogCard`s in skeleton form: the real card surface with the
 * standard stack of logo, title, meta, description, facts and actions. Cards that lay themselves
 * out differently (a row-card grid, an inset body) pass the same `className` the real card gets,
 * and `children` to replace the standard stack with their own placeholders; the card surface is
 * the real one either way. Siblings, so put them in the real grid and wrap the area in a
 * `SkeletonRegion`.
 */
function CatalogCardSkeleton({
  count = 1,
  descriptionLines = 2,
  facts = true,
  actions = true,
  className,
  children,
  ...props
}: CatalogCardSkeletonProps) {
  return (
    <>
      {skeletonSlots("catalog-card-skeleton", count).map(slot => (
        <CatalogCard
          key={slot.id}
          aria-hidden="true"
          className={cn("animate-skeleton-reveal", className)}
          {...props}
        >
          {children ?? (
            <>
              <div className="flex items-start gap-3">
                <CatalogCard.Logo>
                  <Skeleton className="size-full rounded-sm" />
                </CatalogCard.Logo>
                <div className="flex min-w-0 flex-1 flex-col gap-1">
                  <CatalogCard.Title>
                    <SkeletonText className={cycleByPosition(TITLE_WIDTHS, slot.position)} />
                  </CatalogCard.Title>
                  <CatalogCard.Meta>
                    <SkeletonText className="w-12" />
                    <SkeletonText className="w-16" />
                  </CatalogCard.Meta>
                </div>
              </div>
              {descriptionLines > 0 ? (
                <CatalogCard.Description>
                  <SkeletonText lines={descriptionLines} />
                </CatalogCard.Description>
              ) : null}
              {facts ? (
                <p className="text-eyebrow">
                  <SkeletonText className="w-40" />
                </p>
              ) : null}
              {actions ? (
                <CatalogCard.Actions>
                  <Skeleton className="h-button-sm w-16 rounded-pill" />
                </CatalogCard.Actions>
              ) : null}
            </>
          )}
        </CatalogCard>
      ))}
    </>
  );
}

export interface TableRowSkeletonColumn {
  /** Extra classes for the placeholder in this column, typically a width such as `w-24`. */
  className?: string;
  /** Pushes the placeholder to the cell's end for right-aligned columns. */
  align?: "start" | "end";
  /**
   * Classes for this column's `TableCell`: what the real cell carries on top of the table's own
   * cell style, such as its padding, a fixed width, or the responsive display that hides the column
   * in a narrow pane. A column the header hides needs the same display class here, or its
   * placeholder cell would stay and push the visible columns out of line.
   */
  cellClassName?: string;
  /**
   * The cell's own placeholder, for a column whose shape is more than one text bar (a glyph beside
   * a label, an avatar, a title with a meta line). It receives the row's position, so a stack of
   * rows can vary its widths, and replaces the default bar: `className` and `align` do not apply.
   */
  placeholder?: (row: number) => React.ReactNode;
}

export interface TableRowSkeletonProps extends Omit<React.ComponentProps<"tr">, "children"> {
  /** Rows to render. Defaults to 5. */
  count?: number;
  /** One entry per column, or just the column count to get generic widths. */
  columns: number | readonly TableRowSkeletonColumn[];
}

/**
 * `count` placeholder `TableRow`s of real `TableCell`s, for a table body. Put them inside the
 * real `<TableBody>` of the real `<Table>` (keep the real header) so the cell height, padding
 * and type are the table's own, and wrap the whole table in a `SkeletonRegion`: a table body
 * cannot hold a status region of its own. Give columns the widths the real data has, and the
 * same fixed layout, so the columns do not shift when the rows arrive. A table whose columns
 * come and go with its pane's width passes each column the same `cellClassName` its real cells
 * and header carry.
 */
function TableRowSkeleton({ count = 5, columns, className, ...props }: TableRowSkeletonProps) {
  const cells: readonly TableRowSkeletonColumn[] =
    typeof columns === "number" ? Array.from({ length: columns }, () => ({})) : columns;

  return (
    <>
      {skeletonSlots("table-row-skeleton", count).map(row => (
        <TableRow
          key={row.id}
          aria-hidden="true"
          className={cn("animate-skeleton-reveal hover:bg-transparent", className)}
          {...props}
        >
          {skeletonSlots(`${row.id}-cell`, cells.length).map(cell => {
            const column = cells[cell.position];
            return (
              <TableCell key={cell.id} className={column.cellClassName}>
                {column.placeholder ? (
                  column.placeholder(row.position)
                ) : (
                  <SkeletonText
                    className={cn(
                      cycleByPosition(CELL_WIDTHS, cell.position + row.position),
                      column.align === "end" && "ml-auto",
                      column.className
                    )}
                  />
                )}
              </TableCell>
            );
          })}
        </TableRow>
      ))}
    </>
  );
}

export interface PropertyRowSkeletonProps extends Omit<React.ComponentProps<"div">, "children"> {
  /** Rows to render. Defaults to 4. */
  count?: number;
  /** The `PropertyRow` variant the real inspector uses. */
  variant?: PropertyRowVariant;
}

/**
 * `count` `PropertyRow`s in skeleton form for detail inspectors and fact lists: the real row,
 * with a placeholder label and value. Siblings, so put them in the real rail or list and wrap
 * the area in a `SkeletonRegion`.
 */
function PropertyRowSkeleton({
  count = 4,
  variant,
  className,
  ...props
}: PropertyRowSkeletonProps) {
  return (
    <>
      {skeletonSlots("property-row-skeleton", count).map(slot => (
        <PropertyRow
          key={slot.id}
          aria-hidden="true"
          variant={variant}
          className={cn("animate-skeleton-reveal", className)}
          label={<SkeletonText className={cycleByPosition(LABEL_WIDTHS, slot.position)} />}
          {...props}
        >
          <SkeletonText className={cycleByPosition(VALUE_WIDTHS, slot.position)} />
        </PropertyRow>
      ))}
    </>
  );
}

export {
  CatalogCardSkeleton,
  ItemSkeleton,
  ListingRowSkeleton,
  PropertyRowSkeleton,
  TableRowSkeleton,
};
