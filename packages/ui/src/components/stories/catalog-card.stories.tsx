import { AlertCircle, Download, Wrench } from "lucide-react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { Button } from "../button";
import { CatalogCard } from "../custom/catalog-card";
import { Pill } from "../custom/pill";
import { Skeleton } from "../skeleton";

const meta: Meta<typeof CatalogCard> = {
  title: "components/custom/CatalogCard",
  component: CatalogCard,
  args: {},
};

export default meta;
type Story = StoryObj<typeof meta>;

interface CatalogCardExampleProps {
  installed?: boolean;
}

function CatalogCardExample({ installed = false }: CatalogCardExampleProps) {
  return (
    <CatalogCard className="max-w-sm" data-testid="catalog-card-story">
      <div className="flex items-start gap-3">
        <CatalogCard.Logo>
          <Wrench className="size-4" />
        </CatalogCard.Logo>
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <CatalogCard.Title>release-auditor</CatalogCard.Title>
          <CatalogCard.Meta>
            <span>@platform</span>
            <span>v1.4.2</span>
            <span className="inline-flex items-center gap-1">
              <Download aria-hidden="true" className="size-3" />
              287
            </span>
          </CatalogCard.Meta>
        </div>
      </div>
      <CatalogCard.Description>
        Checks release notes, version policy, and operator-facing docs before a tagged build.
      </CatalogCard.Description>
      <div className="flex flex-wrap items-center gap-1.5">
        <Pill mono tone="neutral">
          release
        </Pill>
        <Pill mono tone="neutral">
          docs
        </Pill>
      </div>
      <CatalogCard.Actions>
        {installed ? (
          <Pill mono tone="success">
            INSTALLED
          </Pill>
        ) : (
          <Button size="sm" type="button" variant="outline">
            Install
          </Button>
        )}
      </CatalogCard.Actions>
    </CatalogCard>
  );
}

export const Default: Story = {
  render: () => <CatalogCardExample />,
};

export const WithMeta: Story = {
  args: {},
  render: () => (
    <div className="grid max-w-3xl grid-cols-1 gap-3 sm:grid-cols-2">
      <CatalogCardExample installed />
      <CatalogCardExample />
    </div>
  ),
};

export const WithActions: Story = {
  args: {},
  render: () => (
    <CatalogCard className="max-w-sm" actionable>
      <div className="flex items-start gap-3">
        <CatalogCard.Logo tone="info">
          <Wrench className="size-4" />
        </CatalogCard.Logo>
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <CatalogCard.Title>policy-reader</CatalogCard.Title>
          <CatalogCard.Meta>
            <span>@governance</span>
            <span>v0.9.8</span>
          </CatalogCard.Meta>
        </div>
      </div>
      <CatalogCard.Description>
        Provides a read-only view of policy metadata exposed by the runtime.
      </CatalogCard.Description>
      <CatalogCard.Actions>
        <Button size="sm" type="button" variant="outline">
          Inspect
        </Button>
        <Pill mono tone="neutral">
          READ ONLY
        </Pill>
      </CatalogCard.Actions>
    </CatalogCard>
  ),
};

export const Loading: Story = {
  args: {},
  render: () => (
    <CatalogCard className="max-w-sm" aria-label="Loading catalog card">
      <div className="flex items-start gap-3">
        <Skeleton className="size-9 rounded-lg" />
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <Skeleton className="h-4 w-40" />
          <Skeleton className="h-3 w-28" />
        </div>
      </div>
      <Skeleton className="h-12 w-full" />
      <CatalogCard.Actions>
        <Skeleton className="h-8 w-20" />
      </CatalogCard.Actions>
    </CatalogCard>
  ),
};

export const Error: Story = {
  args: {},
  render: () => (
    <CatalogCard className="max-w-sm" data-testid="catalog-card-error">
      <div className="flex items-start gap-3">
        <CatalogCard.Logo tone="danger">
          <AlertCircle className="size-4" />
        </CatalogCard.Logo>
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <CatalogCard.Title>catalog unavailable</CatalogCard.Title>
          <CatalogCard.Meta>
            <span>runtime</span>
          </CatalogCard.Meta>
        </div>
      </div>
      <CatalogCard.Description>
        The catalog endpoint returned an error. Existing installed capabilities are unchanged.
      </CatalogCard.Description>
      <CatalogCard.Actions>
        <Button size="sm" type="button" variant="outline">
          Retry
        </Button>
      </CatalogCard.Actions>
    </CatalogCard>
  ),
};

/**
 * Selected state — `--surface-glaze` + 1 px inset `--line-strong` ring No accent.
 */
export const Selected: Story = {
  args: {},
  render: () => (
    <CatalogCard className="max-w-sm" data-testid="catalog-card-selected" selected>
      <div className="flex items-start gap-3">
        <CatalogCard.Logo>
          <Wrench className="size-4" />
        </CatalogCard.Logo>
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <CatalogCard.Title>release-auditor</CatalogCard.Title>
          <CatalogCard.Meta>
            <span>@platform</span>
            <span>v1.4.2</span>
          </CatalogCard.Meta>
        </div>
      </div>
    </CatalogCard>
  ),
};

/**
 * `logoSize="lg"` (40 × 40) for connected/configured provider surfaces.
 */
export const ConfiguredProvider: Story = {
  args: {},
  render: () => (
    <CatalogCard className="max-w-sm" data-testid="catalog-card-provider">
      <div className="flex items-start gap-3">
        <CatalogCard.Logo size="lg" tone="info">
          <Wrench className="size-5" />
        </CatalogCard.Logo>
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <CatalogCard.Title>Anthropic</CatalogCard.Title>
          <CatalogCard.Meta>
            <span>configured</span>
            <span>claude-opus-4-7</span>
          </CatalogCard.Meta>
        </div>
      </div>
    </CatalogCard>
  ),
};
