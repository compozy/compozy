import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";

import { BoxesIcon, Wrench } from "../../../icons";
import { cn } from "../../../lib/utils";
import { Button } from "../../button";
import { Item, ItemActions, ItemContent, ItemDescription, ItemMedia, ItemTitle } from "../../item";
import { SkeletonRegion } from "../../skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../table";
import { CatalogCard } from "../catalog-card";
import {
  CatalogCardSkeleton,
  ItemSkeleton,
  ListingRowSkeleton,
  PropertyRowSkeleton,
  TableRowSkeleton,
} from "../content-skeleton";
import { Eyebrow } from "../eyebrow";
import { ListingRow } from "../listing-row";
import { PropertyRow } from "../property-row";

const meta: Meta = {
  title: "components/custom/ContentSkeleton",
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Skeletons that mirror a real primitive. Each renders the real container and slot components and fills only the leaves with placeholders, so row heights, paddings, gaps, icon wells and line boxes come from the real classes and cannot drift. Every story shows the skeleton beside the loaded content in the same frame: the two sides measure the same height. The shapes are presentational and hidden from assistive technology; wrap the loading area in one `SkeletonRegion`. A skeleton lays out at once but stays transparent for one `base` beat, so give a fresh story a moment before it appears.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

interface CompareProps {
  loading: ReactNode;
  loaded: ReactNode;
}

/** Skeleton and loaded content in one frame, in identical containers. */
function Compare({ loading, loaded }: CompareProps) {
  return (
    <div className="grid w-[1040px] grid-cols-2 items-start gap-6">
      <div className="flex min-w-0 flex-col gap-2" data-testid="skeleton-side">
        <Eyebrow>Skeleton</Eyebrow>
        <SkeletonRegion label="Loading">{loading}</SkeletonRegion>
      </div>
      <div className="flex min-w-0 flex-col gap-2" data-testid="loaded-side">
        <Eyebrow>Loaded</Eyebrow>
        <div>{loaded}</div>
      </div>
    </div>
  );
}

const CARD_SURFACE = "overflow-hidden rounded-lg bg-card shadow-card";

const LISTING_ROWS = [
  { name: "alpha-delivery", description: "Ships a release candidate through the gate." },
  { name: "beta-watch", description: "Watches the beta channel and reports regressions." },
  { name: "gamma-audit", description: "Audits docs and version policy before a tag." },
] as const;

function LoadedListingRows() {
  return (
    <div className={CARD_SURFACE}>
      {LISTING_ROWS.map(row => (
        <ListingRow key={row.name} interactive={false}>
          <ListingRow.Icon>
            <BoxesIcon aria-hidden="true" className="size-4" />
          </ListingRow.Icon>
          <ListingRow.Main>
            <ListingRow.Name>
              <ListingRow.Title>{row.name}</ListingRow.Title>
            </ListingRow.Name>
            <ListingRow.Description>{row.description}</ListingRow.Description>
            <ListingRow.Meta>
              <span>3 inputs</span>
              <span>iteration cap 8</span>
            </ListingRow.Meta>
          </ListingRow.Main>
          <ListingRow.Trail>
            <ListingRow.Stat>
              <ListingRow.Stat.Value>92%</ListingRow.Stat.Value>
              <ListingRow.Stat.Label>12 runs · 30d</ListingRow.Stat.Label>
            </ListingRow.Stat>
          </ListingRow.Trail>
        </ListingRow>
      ))}
    </div>
  );
}

export const ListingRows: Story = {
  render: () => (
    <Compare
      loading={
        <div className={CARD_SURFACE}>
          <ListingRowSkeleton count={3} description />
        </div>
      }
      loaded={<LoadedListingRows />}
    />
  ),
};

export const ListingRowsCompact: Story = {
  name: "Listing rows (no description, pill trail)",
  render: () => (
    <Compare
      loading={
        <div className={CARD_SURFACE}>
          <ListingRowSkeleton count={3} trail="pill" />
        </div>
      }
      loaded={
        <div className={CARD_SURFACE}>
          {LISTING_ROWS.map(row => (
            <ListingRow key={row.name} interactive={false}>
              <ListingRow.Icon>
                <BoxesIcon aria-hidden="true" className="size-4" />
              </ListingRow.Icon>
              <ListingRow.Main>
                <ListingRow.Name>
                  <ListingRow.Title>{row.name}</ListingRow.Title>
                </ListingRow.Name>
                <ListingRow.Meta>
                  <span>3 inputs</span>
                  <span>iteration cap 8</span>
                </ListingRow.Meta>
              </ListingRow.Main>
              <ListingRow.Trail>
                <Button size="sm" type="button" variant="outline">
                  Run
                </Button>
              </ListingRow.Trail>
            </ListingRow>
          ))}
        </div>
      }
    />
  ),
};

const ITEMS = [
  { title: "Claude Code", description: "Primary orchestrator for long sessions." },
  { title: "Codex CLI", description: "Fast fallback for single-shot refactors." },
  { title: "Gemini CLI", description: "Bound to the docs search workspace." },
] as const;

/**
 * `ItemGroup` wraps each child in its own list item, so inside one pass a single
 * `<ItemSkeleton count={1} />` per row; elsewhere `count` renders the rows as siblings.
 */
export const Items: Story = {
  render: () => (
    <Compare
      loading={
        <div className="flex flex-col gap-1">
          <ItemSkeleton count={3} actions variant="outline" />
        </div>
      }
      loaded={
        <div className="flex flex-col gap-1">
          {ITEMS.map(item => (
            <Item key={item.title} variant="outline">
              <ItemMedia variant="icon">
                <Wrench aria-hidden="true" />
              </ItemMedia>
              <ItemContent>
                <ItemTitle>{item.title}</ItemTitle>
                <ItemDescription>{item.description}</ItemDescription>
              </ItemContent>
              <ItemActions>
                <Button size="sm" type="button" variant="outline">
                  Open
                </Button>
              </ItemActions>
            </Item>
          ))}
        </div>
      }
    />
  ),
};

export const ItemsCompact: Story = {
  name: "Items (xs, no description)",
  render: () => (
    <Compare
      loading={
        <div className="flex flex-col">
          <ItemSkeleton count={4} size="xs" description={false} media="none" />
        </div>
      }
      loaded={
        <div className="flex flex-col">
          {["main", "release/v0.4", "feat/skeletons", "fix/gate"].map(branch => (
            <Item key={branch} size="xs">
              <ItemContent>
                <ItemTitle>{branch}</ItemTitle>
              </ItemContent>
            </Item>
          ))}
        </div>
      }
    />
  ),
};

function LoadedCatalogCards() {
  return (
    <>
      {["release-auditor", "policy-reader"].map(name => (
        <CatalogCard key={name}>
          <div className="flex items-start gap-3">
            <CatalogCard.Logo>
              <Wrench aria-hidden="true" className="size-4" />
            </CatalogCard.Logo>
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <CatalogCard.Title>{name}</CatalogCard.Title>
              <CatalogCard.Meta>
                <span>@platform</span>
                <span>v1.4.2</span>
              </CatalogCard.Meta>
            </div>
          </div>
          <CatalogCard.Description>
            Checks release notes, version policy and operator-facing docs before a tagged build,
            then reports anything that needs a human.
          </CatalogCard.Description>
          <p className="text-eyebrow text-faint">287 installs · updated 2h ago</p>
          <CatalogCard.Actions>
            <Button size="sm" type="button" variant="outline">
              Install
            </Button>
          </CatalogCard.Actions>
        </CatalogCard>
      ))}
    </>
  );
}

export const CatalogCards: Story = {
  render: () => (
    <Compare
      loading={
        <div className="grid grid-cols-1 gap-3">
          <CatalogCardSkeleton count={2} />
        </div>
      }
      loaded={
        <div className="grid grid-cols-1 gap-3">
          <LoadedCatalogCards />
        </div>
      }
    />
  ),
};

const COLUMN_WIDTHS = ["w-48", "w-24", "w-28", "w-20"] as const;
const TABLE_ROWS = [
  ["sess_01", "claude-code", "running", "2m 14s"],
  ["sess_02", "codex", "paused", "0m 42s"],
  ["sess_03", "gemini-cli", "completed", "6m 03s"],
] as const;

/** The columns a narrow pane drops: display classes the header, rows and placeholders all carry. */
const NARROW_HIDDEN = "hidden @2xl:table-cell";

function SessionTableHead({ narrowHides = false }: { narrowHides?: boolean }) {
  return (
    <TableHeader>
      <TableRow>
        {["ID", "Agent", "Status", "Duration"].map((label, index) => (
          <TableHead
            key={label}
            className={cn(COLUMN_WIDTHS[index], narrowHides && index >= 2 && NARROW_HIDDEN)}
          >
            {label}
          </TableHead>
        ))}
      </TableRow>
    </TableHeader>
  );
}

/** Keep the real header and the real fixed layout; give the columns the widths the data has. */
export const TableRows: Story = {
  render: () => (
    <Compare
      loading={
        <div className={CARD_SURFACE}>
          <Table className="table-fixed">
            <SessionTableHead />
            <TableBody>
              <TableRowSkeleton
                count={3}
                columns={[
                  { className: "w-24" },
                  { className: "w-20" },
                  { className: "w-16" },
                  { className: "w-12" },
                ]}
              />
            </TableBody>
          </Table>
        </div>
      }
      loaded={
        <div className={CARD_SURFACE}>
          <Table className="table-fixed">
            <SessionTableHead />
            <TableBody>
              {TABLE_ROWS.map(row => (
                <TableRow key={row[0]}>
                  {row.map(cell => (
                    <TableCell key={cell}>{cell}</TableCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      }
    />
  ),
};

/**
 * A table whose columns come and go with its pane: below `@2xl` Status and Duration drop out of the
 * header, the rows and the placeholder rows alike, because each placeholder column carries the same
 * `cellClassName` its real cells do. The two sides stay the same height and the same columns.
 */
export const TableRowsNarrowPane: Story = {
  name: "Table rows (columns hidden in a narrow pane)",
  render: () => (
    <Compare
      loading={
        <div className={cn(CARD_SURFACE, "@container")}>
          <Table className="table-fixed">
            <SessionTableHead narrowHides />
            <TableBody>
              <TableRowSkeleton
                count={3}
                columns={[
                  { className: "w-24" },
                  { className: "w-20" },
                  { cellClassName: NARROW_HIDDEN, className: "w-16" },
                  { cellClassName: NARROW_HIDDEN, className: "w-12" },
                ]}
              />
            </TableBody>
          </Table>
        </div>
      }
      loaded={
        <div className={cn(CARD_SURFACE, "@container")}>
          <Table className="table-fixed">
            <SessionTableHead narrowHides />
            <TableBody>
              {TABLE_ROWS.map(row => (
                <TableRow key={row[0]}>
                  {row.map((cell, index) => (
                    <TableCell key={cell} className={index >= 2 ? NARROW_HIDDEN : undefined}>
                      {cell}
                    </TableCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      }
    />
  ),
};

export const PropertyRows: Story = {
  render: () => (
    <Compare
      loading={
        <div className="w-72">
          <PropertyRowSkeleton count={4} />
        </div>
      }
      loaded={
        <div className="w-72">
          {[
            ["Status", "Running"],
            ["Agent", "claude-code"],
            ["Workspace", "compozy"],
            ["Started", "2m ago"],
          ].map(([label, value]) => (
            <PropertyRow key={label} label={label}>
              {value}
            </PropertyRow>
          ))}
        </div>
      }
    />
  ),
};
