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
} from "@agh/ui";
import { createElement, useState } from "react";

import { deriveMCPManagementFilter } from "@/systems/settings";
import { SkillActivationPill, SkillActivationReasons } from "@/systems/skill";

import type { MarketplaceInstalledItem } from "../hooks/use-marketplace-kind-page";
import { marketplaceMCPInstalledStatus } from "../lib/mcp-installed-status";
import type { MarketplaceKind, MarketplaceListing } from "../types";
import { marketplaceKindIcon } from "./marketplace-ui";

interface MarketplaceInstalledCardProps {
  item: MarketplaceInstalledItem;
  pending?: boolean;
  onAction: (entry: MarketplaceListing) => void;
  onAuthorize: (item: MarketplaceInstalledItem) => void;
  onEditMCP: (server: NonNullable<MarketplaceInstalledItem["mcpServer"]>) => void;
  onRemove: (item: MarketplaceInstalledItem) => Promise<void> | void;
  onDeactivate: (item: MarketplaceInstalledItem) => Promise<void> | void;
  onToggleEnabled: (item: MarketplaceInstalledItem, enabled: boolean) => void;
  onUpdateBundle: (item: MarketplaceInstalledItem) => void;
}

function MarketplaceInstalledCard({
  item,
  pending = false,
  onAction,
  onAuthorize,
  onEditMCP,
  onRemove,
  onDeactivate,
  onToggleEnabled,
  onUpdateBundle,
}: MarketplaceInstalledCardProps) {
  const { entry } = item;
  const kind = entry.kind as "skill" | "extension" | "bundle" | "mcp";
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmPending, setConfirmPending] = useState(false);
  const [confirmError, setConfirmError] = useState<string | null>(null);
  const isBundle = kind === "bundle";
  const isMcp = kind === "mcp";
  const detailKind = kind === "bundle" ? "bundle" : kind;
  const mcpStatus = item.mcpServer ? marketplaceMCPInstalledStatus(item.mcpServer) : null;
  const authorizeCta = mcpStatus?.authorize === true;
  const installedName =
    entry.installed_name ?? item.mcpServer?.name ?? item.skill?.name ?? entry.name;
  const mcpManagement = item.mcpServer ? deriveMCPManagementFilter(item.mcpServer) : null;
  const detailSearch = isBundle
    ? undefined
    : {
        installed_name: installedName,
        ...(isMcp && mcpManagement
          ? {
              scope: mcpManagement.scope,
              workspace_id:
                mcpManagement.scope === "workspace" ? mcpManagement.workspace_id : undefined,
            }
          : {}),
      };

  const handleConfirm = async () => {
    setConfirmPending(true);
    setConfirmError(null);
    try {
      if (isBundle) await onDeactivate(item);
      else await onRemove(item);
      setConfirmOpen(false);
    } catch (error) {
      setConfirmError(error instanceof Error ? error.message : "Action failed");
    }
    setConfirmPending(false);
  };

  return (
    <>
      <CatalogCard
        className={pending ? "opacity-55" : undefined}
        data-testid={`marketplace-installed-card-${entry.entry_id}`}
        actionable={!pending}
      >
        {isBundle && item.activationId ? (
          <Link
            aria-disabled={pending || undefined}
            aria-label={`Open ${entry.name} activation`}
            className="flex min-w-0 flex-col gap-3 rounded focus-visible:outline-none focus-visible:shadow-focus-ring"
            onClick={event => {
              if (pending) event.preventDefault();
            }}
            params={{ id: item.activationId }}
            tabIndex={pending ? -1 : undefined}
            to="/marketplace/bundles/activations/$id"
          >
            <InstalledCardHead entry={entry} item={item} kind={kind} />
          </Link>
        ) : (
          <Link
            aria-disabled={pending || undefined}
            aria-label={`View ${entry.name} details`}
            className="flex min-w-0 flex-col gap-3 rounded focus-visible:outline-none focus-visible:shadow-focus-ring"
            onClick={event => {
              if (pending) event.preventDefault();
            }}
            params={{ entryId: entry.entry_id, kind: detailKind }}
            search={detailSearch}
            tabIndex={pending ? -1 : undefined}
            to="/marketplace/$kind/$entryId"
          >
            <InstalledCardHead entry={entry} item={item} kind={kind} />
          </Link>
        )}

        <CatalogCard.Actions>
          <InstalledPills item={item} mcpStatus={mcpStatus} />
          <div className="ml-auto flex items-center gap-1.5">
            {kind === "extension" ? (
              <Switch
                aria-label={`Enable ${entry.name}`}
                checked={item.extensionEnabled === true}
                disabled={pending}
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
            {entry.update_available ? (
              <Button
                disabled={pending}
                onClick={() => (isBundle ? onUpdateBundle(item) : onAction(entry))}
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
                      params={
                        isBundle && item.activationId
                          ? { id: item.activationId }
                          : { entryId: entry.entry_id, kind: detailKind }
                      }
                      search={isBundle && item.activationId ? undefined : detailSearch}
                      to={
                        isBundle && item.activationId
                          ? "/marketplace/bundles/activations/$id"
                          : "/marketplace/$kind/$entryId"
                      }
                    />
                  }
                >
                  View details
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                {item.viaBundle ? (
                  <>
                    <div className="max-w-55 px-2 py-1.5 text-form-hint leading-snug text-subtle">
                      Managed by the <span className="font-medium text-fg">{item.viaBundle}</span>{" "}
                      bundle activation.
                    </div>
                    {item.activationId ? (
                      <DropdownMenuItem
                        render={
                          <Link
                            params={{ id: item.activationId }}
                            to="/marketplace/bundles/activations/$id"
                          />
                        }
                      >
                        Open bundle activation
                      </DropdownMenuItem>
                    ) : null}
                  </>
                ) : (
                  <DropdownMenuItem
                    className="text-danger"
                    onClick={() => {
                      setConfirmError(null);
                      setConfirmOpen(true);
                    }}
                  >
                    {isBundle ? "Deactivate…" : "Remove…"}
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </CatalogCard.Actions>
      </CatalogCard>

      <ConfirmDialog
        cancelLabel="Cancel"
        confirmButtonProps={{ "data-testid": `marketplace-confirm-${entry.entry_id}` }}
        confirmLabel={isBundle ? "Deactivate" : "Remove"}
        confirmTyping={isBundle ? undefined : entry.name}
        description={
          isBundle
            ? "Stops this activation. Installed items stay on disk; only the activation binding is removed."
            : isMcp
              ? "Removes the server from MCP config. Sessions can no longer reach its tools."
              : "Removes the installed files from this daemon. It stays available in the marketplace."
        }
        error={confirmError}
        isPending={confirmPending}
        note={
          isBundle
            ? `Agents, jobs, and triggers from the ${item.profileName ?? "selected"} profile stop receiving new work from this activation.`
            : `Type the ${kind} name to confirm.`
        }
        noteTone={isBundle ? "warning" : "neutral"}
        onConfirm={handleConfirm}
        onOpenChange={setConfirmOpen}
        open={confirmOpen}
        title={`${isBundle ? "Deactivate" : "Remove"} ${entry.name}`}
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
            {entry.kind !== "mcp" && entry.version ? (
              <span className="normal-case font-mono tracking-normal">{`v${entry.version}`}</span>
            ) : null}
            {item.profileName ? <span>{`profile ${item.profileName}`}</span> : null}
            {entry.kind !== "mcp" && item.scopeLabel ? <span>{item.scopeLabel}</span> : null}
            {item.skill?.source ? <span>{item.skill.source}</span> : null}
            {entry.kind === "extension" && entry.trust?.decision ? (
              <span>{entry.trust.decision === "verified" ? "verified" : "extension"}</span>
            ) : null}
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
  return (
    <>
      {item.viaBundle ? <Pill mono>{`via ${item.viaBundle}`}</Pill> : null}
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
      {entry.kind === "extension" && entry.trust ? (
        <Pill mono tone={entry.trust.decision === "verified" ? "success" : "warning"}>
          {entry.trust.decision === "verified" ? "verified" : "unverified"}
        </Pill>
      ) : null}
      {entry.kind === "bundle" ? (
        <Pill mono tone="success">
          active
        </Pill>
      ) : null}
      {entry.update_available ? (
        <Pill mono tone="warning">
          {entry.version ? `v${entry.version} available` : "update available"}
        </Pill>
      ) : null}
    </>
  );
}

export { MarketplaceInstalledCard };
export type { MarketplaceInstalledCardProps };
