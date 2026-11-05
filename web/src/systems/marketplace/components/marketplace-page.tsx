import { Plus, RefreshCw, Store } from "@compozy/ui/icons";

import {
  Button,
  ListingPage,
  Spinner,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  useTopbarSlot,
} from "@compozy/ui";

import type { MarketplaceSearch } from "../lib/marketplace-search";
import {
  MCPServersManagementBody,
  MCPServersManagementDialogs,
  SettingsPageRestartNotice,
} from "@/systems/settings";
import { MarketplaceAddMenu } from "./marketplace-add-menu";
import { MarketplaceResults } from "./marketplace-results";
import { MARKETPLACE_PAGE_BODY_CLASS } from "./marketplace-classes";
import { useMarketplaceBrowse } from "./use-marketplace-browse";

interface MarketplacePageProps {
  search: MarketplaceSearch;
  liveDataEnabled?: boolean;
}

/**
 * Browse, one kind at a time. Plugins: head = Store · Marketplace · catalog count · Refresh · Add ▾;
 * body = installed shelf → sections by source → grid. MCPs: head = server count · Add MCP server;
 * body = the managed MCP server list Settings shows. Strip = Plugins | MCPs · search.
 */
function MarketplacePage({ search, liveDataEnabled = true }: MarketplacePageProps) {
  const browse = useMarketplaceBrowse(search, liveDataEnabled);
  const { actions, addMarketplace, install, kind, mcp, page, query, strip } = browse;
  const mcpCount = mcp.collection.isPending ? "–" : mcp.servers.length;

  useTopbarSlot({
    glyph: <Store />,
    crumb: "Marketplace",
    // Each kind counts only itself: catalog entries or MCP servers, never a sum.
    count: kind === "mcps" ? mcpCount : page.isLoading && !page.total ? "–" : page.total,
    actions:
      kind === "mcps" ? (
        <Button
          data-testid="marketplace-mcp-create"
          onClick={mcp.openCreate}
          size="sm"
          type="button"
          variant="secondary"
        >
          <Plus aria-hidden="true" />
          Add MCP server
        </Button>
      ) : (
        <>
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  aria-label="Refresh"
                  data-testid="marketplace-refresh"
                  disabled={page.isRefreshing}
                  onClick={() => void page.refresh()}
                  size="icon-sm"
                  type="button"
                  variant="quiet"
                />
              }
            >
              {page.isRefreshing ? (
                <Spinner aria-hidden="true" className="size-3.5" />
              ) : (
                <RefreshCw aria-hidden="true" className="size-3.5" />
              )}
            </TooltipTrigger>
            <TooltipContent>Refresh catalog</TooltipContent>
          </Tooltip>
          <MarketplaceAddMenu
            onAddMarketplace={addMarketplace.open}
            onAddMCPServer={browse.addMCPServer}
            onInstall={install.open}
          />
        </>
      ),
    toolbar: strip.toolbar,
  });

  return (
    <ListingPage data-kind={kind} data-testid="marketplace-page">
      <div className={MARKETPLACE_PAGE_BODY_CLASS}>
        {kind === "mcps" ? (
          <>
            <SettingsPageRestartNotice slug="mcp" />
            <MCPServersManagementBody management={mcp} />
          </>
        ) : (
          <MarketplaceResults
            actions={actions}
            onAddMarketplace={addMarketplace.open}
            onClearSearch={strip.clear}
            onInstallFromGitHub={() => install.open("github")}
            page={page}
            query={query}
          />
        )}
      </div>
      {actions.dialogs}
      {install.dialogs}
      {addMarketplace.dialog}
      <MCPServersManagementDialogs management={mcp} />
      <span aria-live="polite" className="sr-only">
        {query
          ? `Search updated · ${kind === "mcps" ? mcp.filteredServers.length : page.catalogItems.length} results`
          : ""}
      </span>
    </ListingPage>
  );
}

export { MarketplacePage };
export type { MarketplacePageProps };
