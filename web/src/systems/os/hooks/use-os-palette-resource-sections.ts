import { useExtensionInventory } from "@/systems/extensions";
import { useMemories } from "@/systems/knowledge";
import { useMarketplaceSearch } from "@/systems/marketplace";
import { useNetworkChannels } from "@/systems/network";
import { useVaultSecrets } from "@/systems/vault";

import {
  knowledgeRoute,
  marketplaceCatalogTotal,
  marketplaceEntryRoute,
  networkChannelRoute,
  projectVaultRows,
  rowSeed,
  section,
  workspaceLabel,
  type OsPaletteDomainSection,
} from "../lib/os-palette-domain-search";
import {
  paletteDomainEnabled,
  type OsPaletteDomainContext,
} from "../lib/os-palette-domain-context";
import type { OsPaletteWorkspaceCatalogs } from "./use-os-palette-workspace-catalogs";
import { usePaletteInfiniteCatalog } from "./use-palette-infinite-catalog";

const EMPTY_SECTION = (title: string): OsPaletteDomainSection => ({
  title,
  rows: [],
  total: 0,
  loading: false,
  error: null,
});

export function useOsPaletteResourceSections(
  context: OsPaletteDomainContext,
  catalogs: OsPaletteWorkspaceCatalogs
): readonly OsPaletteDomainSection[] {
  return [
    useKnowledgeSection(context, catalogs),
    useVaultSection(context),
    useNetworkSection(context, catalogs),
    useMarketplaceSection(context),
    useExtensionSection(context, catalogs),
  ];
}

function useKnowledgeSection(
  context: OsPaletteDomainContext,
  catalogs: OsPaletteWorkspaceCatalogs
) {
  const workspaceEnabled =
    paletteDomainEnabled(context, "Knowledge") && context.scopedWorkspace !== null;
  const globalEnabled = paletteDomainEnabled(context, "Knowledge") && context.scope === "global";
  const globalMemories = useMemories(
    { profile: context.profile, scope: "profile" },
    { enabled: globalEnabled }
  );
  const workspaceMemories = useMemories(
    {
      profile: context.profile,
      scope: "workspace",
      workspaceId: context.scopedWorkspace ?? "",
    },
    { enabled: workspaceEnabled }
  );
  usePaletteInfiniteCatalog(globalMemories, globalEnabled);
  usePaletteInfiniteCatalog(workspaceMemories, workspaceEnabled);
  if (context.signals === null) return EMPTY_SECTION("Knowledge");
  const memories =
    context.scope === "global"
      ? [...(globalMemories.data ?? []), ...catalogs.workspaceMemories]
      : (workspaceMemories.data ?? []);
  const total =
    context.scope === "global"
      ? globalMemories.total + catalogs.workspaceMemoryTotal
      : workspaceMemories.total;
  return section(
    "Knowledge",
    memories.map(memory =>
      rowSeed("Knowledge", {
        key: `knowledge:${memory.scope}:${memory.workspace_id ?? ""}:${memory.filename}`,
        label: memory.name,
        detail: memory.description,
        workspaceLabel: workspaceLabel(context.scope, memory.workspace_id, context.workspaceNames),
        app: "knowledge",
        route: knowledgeRoute({
          filename: memory.filename,
          scope: memory.scope === "workspace" ? "workspace" : "global",
          workspaceId: memory.workspace_id,
        }),
        workspaceId: memory.scope === "workspace" ? memory.workspace_id : undefined,
      })
    ),
    {
      isLoading:
        context.scope === "global"
          ? globalMemories.isLoading || catalogs.workspaceMemoryState.isLoading
          : workspaceMemories.isLoading,
      isError:
        context.scope === "global"
          ? globalMemories.isError || catalogs.workspaceMemoryState.isError
          : workspaceMemories.isError,
      error:
        context.scope === "global"
          ? (globalMemories.error ?? catalogs.workspaceMemoryState.error)
          : workspaceMemories.error,
    },
    paletteDomainEnabled(context, "Knowledge"),
    context.query,
    context.signals,
    { limit: context.domainLimit, catalogTotal: total }
  );
}

function useVaultSection(context: OsPaletteDomainContext) {
  const enabled = paletteDomainEnabled(context, "Vault");
  const vault = useVaultSecrets({}, { enabled });
  if (context.signals === null) return EMPTY_SECTION("Vault");
  const rows = projectVaultRows(vault.data ?? [], context.scope, context.workspaceNames);
  return section(
    "Vault",
    rows.map(row => rowSeed("Vault", row, [row.namespace, row.kind])),
    vault,
    enabled,
    context.query,
    context.signals,
    { limit: context.domainLimit, catalogTotal: rows.length }
  );
}

function useNetworkSection(context: OsPaletteDomainContext, catalogs: OsPaletteWorkspaceCatalogs) {
  const workspaceEnabled =
    paletteDomainEnabled(context, "Network channels") && Boolean(context.scopedWorkspace);
  const globalEnabled =
    paletteDomainEnabled(context, "Network channels") && context.scope === "global";
  const channels = useNetworkChannels({
    enabled: workspaceEnabled,
    workspaceId: context.scopedWorkspace,
  });
  if (context.signals === null) return EMPTY_SECTION("Network channels");
  const rows = context.scope === "global" ? catalogs.channels : channels.channels;
  return section(
    "Network channels",
    rows.flatMap(channel => {
      const workspaceId = channel.workspace_id ?? context.scopedWorkspace;
      if (!workspaceId) return [];
      return [
        rowSeed("Network channels", {
          key: `network-channel:${workspaceId}:${channel.channel}`,
          label: channel.channel,
          detail: channel.purpose,
          workspaceLabel: workspaceLabel(context.scope, workspaceId, context.workspaceNames),
          app: "network",
          route: networkChannelRoute(workspaceId, channel.channel),
          workspaceId,
        }),
      ];
    }),
    context.scope === "global" ? catalogs.channelState : channels,
    workspaceEnabled || globalEnabled,
    context.query,
    context.signals,
    {
      limit: context.domainLimit,
      catalogTotal: context.scope === "global" ? catalogs.channelTotal : channels.channels.length,
    }
  );
}

function useMarketplaceSection(context: OsPaletteDomainContext) {
  const enabled = paletteDomainEnabled(context, "Marketplace");
  const marketplace = useMarketplaceSearch(
    {
      q: context.query,
      ...(context.scopedWorkspace ? { workspaceId: context.scopedWorkspace } : {}),
    },
    enabled
  );
  if (context.signals === null) return EMPTY_SECTION("Marketplace");
  return section(
    "Marketplace",
    (marketplace.data?.kinds ?? []).flatMap(kind =>
      kind.items.map(item =>
        rowSeed("Marketplace", {
          key: `marketplace:${item.kind}:${item.entry_id}`,
          label: item.name,
          detail: item.description,
          workspaceLabel: workspaceLabel(
            context.scope,
            context.scopedWorkspace,
            context.workspaceNames
          ),
          app: "marketplace",
          route: marketplaceEntryRoute({
            kind: item.kind,
            entryId: item.entry_id,
            scope: context.scope,
            workspaceId: context.scopedWorkspace,
            installedName: item.installed_name,
          }),
          ...(context.scopedWorkspace ? { workspaceId: context.scopedWorkspace } : {}),
        })
      )
    ),
    marketplace,
    enabled,
    context.query,
    context.signals,
    {
      limit: context.domainLimit,
      catalogTotal: marketplaceCatalogTotal(marketplace.data?.kinds),
    }
  );
}

function useExtensionSection(
  context: OsPaletteDomainContext,
  catalogs: OsPaletteWorkspaceCatalogs
) {
  const enabled = paletteDomainEnabled(context, "Extensions");
  const published = useExtensionInventory(enabled && context.scope === "workspace");
  if (context.signals === null) return EMPTY_SECTION("Extensions");
  const rows =
    context.scope === "global"
      ? [
          ...catalogs.publishedExtensions.map(extension => ({
            extension,
            workspaceId: undefined,
          })),
          ...catalogs.workspaceExtensions,
        ]
      : (published.data ?? []).map(item => ({
          extension: item.extension,
          workspaceId: published.workspaceId ?? undefined,
        }));
  return section(
    "Extensions",
    rows.map(({ extension, workspaceId }) =>
      rowSeed("Extensions", {
        key: `extension:${workspaceId ?? "published"}:${extension.name}`,
        label: extension.name,
        detail: extension.health,
        workspaceLabel: workspaceLabel(context.scope, workspaceId, context.workspaceNames),
        app: "marketplace",
        route: marketplaceEntryRoute({
          kind: "extension",
          entryId: extension.marketplace?.entry_id ?? extension.name,
          scope: workspaceId ? "workspace" : "global",
          workspaceId,
          installedName: extension.name,
        }),
        ...(workspaceId ? { workspaceId } : {}),
      })
    ),
    context.scope === "global" ? catalogs.workspaceExtensionState : published,
    enabled,
    context.query,
    context.signals,
    {
      limit: context.domainLimit,
      catalogTotal:
        context.scope === "global"
          ? catalogs.publishedExtensions.length + catalogs.workspaceExtensions.length
          : published.data?.length,
    }
  );
}
