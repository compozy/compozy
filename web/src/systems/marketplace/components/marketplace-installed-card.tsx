import { Link } from "@tanstack/react-router";
import { MoreHorizontal } from "lucide-react";

import {
  Button,
  CatalogCard,
  ConfirmDialog,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Pill,
  Spinner,
  Switch,
} from "@compozy/ui";
import { createElement } from "react";

import type { MarketplaceInstalledItem } from "../hooks/use-marketplace-kind-page";
import { marketplaceMCPInstalledStatus } from "../lib/mcp-installed-status";
import type { MarketplaceKind, MarketplaceListing } from "../types";
import { formatMarketplaceVersion, marketplaceKindIcon } from "./marketplace-ui";
import { useMarketplaceInstalledCardConfirmation } from "./use-marketplace-installed-card-confirmation";
import { ExtensionFormatBadge, ExtensionTrustBadges } from "@/systems/extensions";
import { deriveMCPManagementFilter } from "@/systems/settings";
import { SkillActivationPill, SkillActivationReasons } from "@/systems/skill";

interface MarketplaceInstalledCardProps {
  item: MarketplaceInstalledItem;
  pending?: boolean;
  onAction: (entry: MarketplaceListing) => void;
  onAuthorize: (item: MarketplaceInstalledItem) => void;
  onEditMCP: (server: NonNullable<MarketplaceInstalledItem["mcpServer"]>) => void;
  onRemove: (item: MarketplaceInstalledItem) => Promise<void> | void;
  onToggleEnabled: (item: MarketplaceInstalledItem, enabled: boolean) => void;
}

function MarketplaceInstalledCard({
  item,
  pending = false,
  onAction,
  onAuthorize,
  onEditMCP,
  onRemove,
  onToggleEnabled,
}: MarketplaceInstalledCardProps) {
  const { entry } = item;
  const kind = entry.kind;
  const confirmation = useMarketplaceInstalledCardConfirmation(() =>
    Promise.resolve(onRemove(item))
  );
  const isMcp = kind === "mcp";
  // A dev overlay shadows the published row without owning it: enable/disable and marketplace
  // updates act on the published extension, so this card must not offer them.
  const isDevOverlay = item.extensionFacts?.dev === true;
  const mcpStatus = item.mcpServer ? marketplaceMCPInstalledStatus(item.mcpServer) : null;
  const authorizeCta = mcpStatus?.authorize === true;
  const installedName =
    entry.installed_name ?? item.mcpServer?.name ?? item.skill?.name ?? entry.name;
  const mcpManagement = item.mcpServer ? deriveMCPManagementFilter(item.mcpServer) : null;
  const detailSearch = {
    installed_name: installedName,
    ...(isMcp && mcpManagement
      ? {
          scope: mcpManagement.scope,
          ...(mcpManagement.scope === "profile" ? { profile: mcpManagement.profile } : {}),
          workspace_id:
            mcpManagement.scope === "workspace" ? mcpManagement.workspace_id : undefined,
        }
      : {}),
  };

  return (
    <>
      <CatalogCard
        className={pending ? "opacity-55" : undefined}
        data-testid={`marketplace-installed-card-${entry.entry_id}`}
        actionable={!pending}
      >
        <Link
          aria-disabled={pending || undefined}
          aria-label={`View ${entry.name} details`}
          className="flex min-w-0 flex-col gap-3 rounded focus-visible:outline-none focus-visible:shadow-focus-ring"
          onClick={event => {
            if (pending) event.preventDefault();
          }}
          params={{ entryId: entry.entry_id, kind }}
          search={detailSearch}
          tabIndex={pending ? -1 : undefined}
          to="/marketplace/$kind/$entryId"
        >
          <InstalledCardHead entry={entry} item={item} kind={kind} />
        </Link>

        <CatalogCard.Actions>
          <InstalledPills item={item} mcpStatus={mcpStatus} />
          <div className="ml-auto flex items-center gap-1.5">
            {kind === "extension" ? (
              <Switch
                aria-label={
                  isDevOverlay
                    ? `Enable ${entry.name} · managed on the published extension`
                    : `Enable ${entry.name}`
                }
                checked={item.extensionEnabled === true}
                disabled={pending || isDevOverlay}
                onCheckedChange={checked => onToggleEnabled(item, checked)}
              />
            ) : null}
            {authorizeCta ? (
              <Button
                disabled={pending}
                onClick={() => onAuthorize(item)}
                size="sm"
                type="button"
                variant="default"
              >
                Authorize
              </Button>
            ) : null}
            {entry.update_available && !isDevOverlay ? (
              <Button
                data-testid={`marketplace-installed-update-${entry.entry_id}`}
                disabled={pending}
                onClick={() => onAction(entry)}
                size="sm"
                type="button"
                variant="neutral"
              >
                {pending ? <Spinner aria-hidden="true" className="size-3" /> : null}
                Update
              </Button>
            ) : null}
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button
                    aria-haspopup="menu"
                    aria-label={`More for ${entry.name}`}
                    disabled={pending}
                    size="icon-sm"
                    type="button"
                    variant="ghost"
                  />
                }
              >
                <MoreHorizontal aria-hidden="true" className="size-3.5" />
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {item.mcpServer ? (
                  <DropdownMenuItem
                    onClick={() => {
                      if (item.mcpServer) onEditMCP(item.mcpServer);
                    }}
                  >
                    Edit configuration
                  </DropdownMenuItem>
                ) : null}
                <DropdownMenuItem
                  render={
                    <Link
                      params={{ entryId: entry.entry_id, kind }}
                      search={detailSearch}
                      to="/marketplace/$kind/$entryId"
                    />
                  }
                >
                  View details
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem className="text-danger" onClick={confirmation.openConfirmation}>
                  Remove…
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </CatalogCard.Actions>
      </CatalogCard>

      <ConfirmDialog
        cancelLabel="Cancel"
        confirmButtonProps={{ "data-testid": `marketplace-confirm-${entry.entry_id}` }}
        confirmLabel="Remove"
        confirmTyping={entry.name}
        description={
          isMcp
            ? "Removes the server from MCP config. Sessions can no longer reach its tools."
            : "Removes the installed files from this machine. It stays available in the marketplace."
        }
        error={confirmation.error}
        isPending={confirmation.isPending}
        note={`Type the ${kind} name to confirm.`}
        noteTone="neutral"
        onConfirm={confirmation.confirm}
        onOpenChange={open => {
          if (!open) confirmation.cancel();
        }}
        open={confirmation.open}
        title={`Remove ${entry.name}`}
        tone="danger"
      />
    </>
  );
}

function InstalledCardHead({
  entry,
  item,
  kind,
}: {
  entry: MarketplaceListing;
  item: MarketplaceInstalledItem;
  kind: MarketplaceKind;
}) {
  const transport = item.mcpServer?.transport || entry.transport;
  const scopeMeta = item.scopeLabel;
  const version = formatMarketplaceVersion(entry.version);

  return (
    <>
      <div className="flex min-w-0 items-start gap-3">
        <CatalogCard.Logo tone={entry.kind === "extension" ? "neutral" : "accent"}>
          {createElement(marketplaceKindIcon(kind), {
            "aria-hidden": true,
            className: "size-3.5",
          })}
        </CatalogCard.Logo>
        <div className="min-w-0 flex-1">
          <CatalogCard.Title>{entry.name}</CatalogCard.Title>
          <CatalogCard.Meta>
            {entry.kind === "mcp" && transport ? (
              <span className="normal-case font-mono tracking-normal">{transport}</span>
            ) : null}
            {entry.kind === "mcp" && scopeMeta ? <span>{scopeMeta}</span> : null}
            {entry.kind !== "mcp" && version ? (
              <span className="normal-case font-mono tracking-normal">{version}</span>
            ) : null}
            {entry.kind !== "mcp" && item.scopeLabel ? <span>{item.scopeLabel}</span> : null}
            {item.skill?.source ? <span>{item.skill.source}</span> : null}
          </CatalogCard.Meta>
        </div>
      </div>
      {entry.description ? (
        <CatalogCard.Description className="line-clamp-2">
          {entry.description}
        </CatalogCard.Description>
      ) : null}
      {item.skill && !item.skill.activation.active ? (
        <SkillActivationReasons
          data-testid={`skill-card-activation-reasons-${item.skill.name}`}
          density="compact"
          reasons={item.skill.activation.reasons ?? []}
        />
      ) : null}
    </>
  );
}

function InstalledPills({
  item,
  mcpStatus,
}: {
  item: MarketplaceInstalledItem;
  mcpStatus: ReturnType<typeof marketplaceMCPInstalledStatus> | null;
}) {
  const { entry } = item;
  const version = formatMarketplaceVersion(entry.version);
  return (
    <>
      {item.skill ? (
        <SkillActivationPill
          active={item.skill.activation.active}
          data-testid={`skill-card-activation-status-${item.skill.name}`}
          mono
        />
      ) : null}
      {mcpStatus ? (
        <Pill mono tone={mcpStatus.tone}>
          {mcpStatus.label}
        </Pill>
      ) : null}
      <ExtensionFormatBadge format={entry.format} />
      {item.extensionFacts ? <ExtensionTrustBadges facts={item.extensionFacts} showSource /> : null}
      {entry.update_available ? (
        <Pill mono tone="warning">
          {version ? `${version} available` : "update available"}
        </Pill>
      ) : null}
    </>
  );
}

export { MarketplaceInstalledCard };
export type { MarketplaceInstalledCardProps };
