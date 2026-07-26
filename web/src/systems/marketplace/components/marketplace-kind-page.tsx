import type { RefObject } from "react";
import { Link } from "@tanstack/react-router";
import { Plus, RefreshCw } from "lucide-react";

import {
  Button,
  ListingPage,
  ListingToolbar,
  PillGroup,
  RouteNav,
  Spinner,
  useTopbarSlot,
} from "@agh/ui";

import { useListingSearchShortcut } from "@/hooks/use-listing-search-shortcut";
import { MCPServerEditor } from "@/systems/settings";
import { MarketplaceDegradedNotice } from "./marketplace-degraded-notice";
import { MarketplaceKindResults } from "./marketplace-kind-results";
import { useMarketplaceActionController } from "./use-marketplace-action-controller";
import { MARKETPLACE_SCOPE_ICONS, marketplaceKindConfig } from "../lib/marketplace-kind-config";
import { useMarketplaceKindPage } from "../hooks/use-marketplace-kind-page";
import { useRefreshMarketplaceCatalog } from "../hooks/use-marketplace-actions";
import { useMarketplaceMCPEditor } from "../hooks/use-marketplace-mcp-editor";
import { marketplaceRouteKindFor, type MarketplaceKind } from "../types";
import type { MarketplaceKindSearch } from "../lib/marketplace-kind-search";
import { MARKETPLACE_KIND_LABEL, MARKETPLACE_KIND_ORDER } from "./marketplace-ui";

const MARKETPLACE_NAV_ITEMS = MARKETPLACE_KIND_ORDER.map(kind => ({
  kind,
  routeKind: marketplaceRouteKindFor(kind),
  label: MARKETPLACE_KIND_LABEL[kind],
  to: `/marketplace/${marketplaceRouteKindFor(kind)}` as const,
}));

interface MarketplaceKindPageProps {
  kind: MarketplaceKind;
  search: MarketplaceKindSearch;
}

function MarketplaceKindPage({ kind, search }: MarketplaceKindPageProps) {
  const searchInputRef = useListingSearchShortcut();
  const page = useMarketplaceKindPage(kind, search);
  return (
    <MarketplaceKindPageBody
      key={page.workspaceId ?? "none"}
      kind={kind}
      page={page}
      searchInputRef={searchInputRef}
    />
  );
}

interface MarketplaceKindPageBodyProps {
  kind: MarketplaceKind;
  page: ReturnType<typeof useMarketplaceKindPage>;
  searchInputRef: RefObject<HTMLInputElement | null>;
}

function MarketplaceKindPageBody({ kind, page, searchInputRef }: MarketplaceKindPageBodyProps) {
  const refresh = useRefreshMarketplaceCatalog();
  const mcpEditor = useMarketplaceMCPEditor({
    enabled: kind === "mcp",
    scope: page.mcpConfigScope,
    servers: page.mcpEditorServers,
    workspaceId: page.workspaceId,
  });
  const actions = useMarketplaceActionController(page.workspaceId, {
    installedItems: page.installedItems,
    onViewInstalled: () => page.setScope("installed"),
  });
  const config = marketplaceKindConfig(kind);
  const ScopeInstalledIcon = MARKETPLACE_SCOPE_ICONS.installed;
  const ScopeMarketIcon = MARKETPLACE_SCOPE_ICONS.market;
  const headCount = page.scope === "market" ? page.marketplaceTotal : page.installedCount;
  const Icon = config.icon;
  const updatesLabel =
    page.updatesAvailable === 1
      ? "1 update available"
      : `${page.updatesAvailable} updates available`;

  useTopbarSlot({
    glyph: <Icon />,
    crumb: config.label,
    count: page.isLoading && !page.marketplaceTotal ? "–" : headCount,
    actions: (
      <Button
        data-testid="marketplace-refresh"
        disabled={refresh.isPending}
        onClick={() => refresh.mutate(kind === "bundle" ? undefined : kind)}
        size="sm"
        type="button"
        variant="ghost"
      >
        {refresh.isPending ? (
          <Spinner aria-hidden="true" className="size-3" />
        ) : (
          <RefreshCw aria-hidden="true" className="size-3" />
        )}
        Refresh
      </Button>
    ),
    nav: (
      <RouteNav aria-label="Marketplace sections" data-testid="marketplace-kind-navigation">
        {MARKETPLACE_NAV_ITEMS.map(item => (
          <RouteNav.Link
            aria-current={item.kind === kind ? "page" : undefined}
            key={item.routeKind}
            render={<Link to={item.to} />}
          >
            {item.label}
          </RouteNav.Link>
        ))}
      </RouteNav>
    ),
    toolbar: (
      <ListingToolbar>
        <ListingToolbar.Leading>
          <ListingToolbar.Search
            aria-label={`Search ${config.label.toLowerCase()}`}
            containerClassName="w-full max-w-105"
            data-testid={`marketplace-kind-search-${kind}`}
            onChange={page.setDraftQuery}
            placeholder={config.searchPlaceholder}
            ref={searchInputRef}
            value={page.draftQuery}
          />
        </ListingToolbar.Leading>
        <ListingToolbar.Trailing>
          <div className="flex flex-wrap items-center gap-2">
            {kind === "mcp" && page.scope === "installed" ? (
              <PillGroup
                aria-label="New MCP server scope"
                data-testid="marketplace-mcp-config-scope"
                items={[
                  {
                    value: "workspace" as const,
                    label: "Workspace",
                    testId: "marketplace-mcp-config-scope-workspace",
                  },
                  {
                    value: "global" as const,
                    label: "Global",
                    testId: "marketplace-mcp-config-scope-global",
                  },
                ]}
                onChange={page.setMCPConfigScope}
                value={page.mcpConfigScope}
              />
            ) : null}
            <PillGroup
              aria-label="Marketplace scope"
              data-testid={`marketplace-scope-${kind}`}
              items={[
                {
                  value: "installed" as const,
                  testId: `marketplace-scope-installed-${kind}`,
                  badge: page.installedCount,
                  label: (
                    <>
                      <ScopeInstalledIcon aria-hidden="true" />
                      Installed
                    </>
                  ),
                },
                {
                  value: "market" as const,
                  testId: `marketplace-scope-market-${kind}`,
                  label: (
                    <>
                      <ScopeMarketIcon aria-hidden="true" />
                      Marketplace
                    </>
                  ),
                },
              ]}
              onChange={page.setScope}
              value={page.scope}
            />
            {kind === "mcp" && page.scope === "installed" ? (
              <Button
                data-testid="marketplace-mcp-add"
                disabled={page.mcpConfigScope === "workspace" && !page.workspaceId}
                onClick={mcpEditor.openCreate}
                size="sm"
                type="button"
              >
                <Plus aria-hidden="true" className="size-3" />
                Add MCP server
              </Button>
            ) : null}
          </div>
        </ListingToolbar.Trailing>
      </ListingToolbar>
    ),
  });

  return (
    <ListingPage data-testid={`marketplace-kind-${kind}`}>
      <p className="mb-3 text-xs text-subtle" data-testid={`marketplace-kind-meta-${kind}`}>
        <span className="font-mono text-mono-id tabular-nums text-muted">
          {page.marketplaceTotal}
        </span>{" "}
        {page.marketplaceTotalExact ? "in the marketplace" : "loaded from the marketplace"}
        <span aria-hidden="true" className="mx-1.5 text-faint">
          ·
        </span>
        <span className="font-mono text-mono-id tabular-nums text-muted">
          {page.installedCount}
        </span>{" "}
        {config.installedNoun}
        {!page.isLoading && page.updatesAvailable > 0 ? (
          <>
            <span aria-hidden="true" className="mx-1.5 text-faint">
              ·
            </span>
            <span className="font-mono text-mono-id tabular-nums text-muted">
              {page.updatesAvailable}
            </span>{" "}
            {updatesLabel.replace(/^\d+\s/, "")}
          </>
        ) : null}
      </p>

      {page.error &&
      !page.marketplaceContinuationError &&
      ((page.scope === "market" && page.marketEntries.length > 0) ||
        (page.scope === "installed" && page.installedItems.length > 0)) ? (
        <MarketplaceDegradedNotice
          hasItems
          label={config.label}
          onRetry={() => page.refetch()}
          scope="kind"
        />
      ) : null}

      <MarketplaceKindResults
        actions={actions}
        config={config}
        kind={kind}
        onEditMCP={mcpEditor.openEdit}
        page={page}
      />

      {actions.dialogs}
      {mcpEditor.editorProps ? <MCPServerEditor {...mcpEditor.editorProps} /> : null}
      <span aria-live="polite" className="sr-only">
        {page.query
          ? `Search updated · ${page.scope === "market" ? page.marketEntries.length : page.installedItems.length} results`
          : ""}
      </span>
    </ListingPage>
  );
}

export { MarketplaceKindPage };
export type { MarketplaceKindPageProps };
