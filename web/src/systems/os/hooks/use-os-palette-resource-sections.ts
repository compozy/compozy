import { useExtensionInventory } from "@/systems/extensions";
import { useMarketplaceCatalog } from "@/systems/marketplace";
import { useVaultSecrets } from "@/systems/vault";

import {
  marketplaceEntryRoute,
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
    useVaultSection(context),
    useMarketplaceSection(context),
    useExtensionSection(context, catalogs),
  ];
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

function useMarketplaceSection(context: OsPaletteDomainContext) {
  const enabled = paletteDomainEnabled(context, "Marketplace");
  const marketplace = useMarketplaceCatalog(
    {
      q: context.query,
      profileName: context.profile,
      ...(context.scopedWorkspace ? { workspaceId: context.scopedWorkspace } : {}),
    },
    enabled
  );
  if (context.signals === null) return EMPTY_SECTION("Marketplace");
  return section(
    "Marketplace",
    (marketplace.data?.pages ?? []).flatMap(page =>
      page.items.map(item =>
        rowSeed("Marketplace", {
          key: `marketplace:${item.source_ref}:${item.entry_id}`,
          label: item.name,
          detail: item.description,
          workspaceLabel: workspaceLabel(
            context.scope,
            context.scopedWorkspace,
            context.workspaceNames
          ),
          app: "marketplace",
          route: marketplaceEntryRoute({
            source: item.source,
            profileName: context.profile,
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
      catalogTotal: marketplace.data?.pages[0]?.total ?? 0,
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
          source: extension.marketplace?.source,
          profileName: context.profile,
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
