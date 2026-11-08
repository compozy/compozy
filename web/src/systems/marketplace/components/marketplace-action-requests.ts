import { toast } from "@compozy/ui";

import type { useExtensionInstanceScope } from "@/systems/extensions";

import type { ExtensionInstallRequest, MarketplaceCatalogListing } from "../types";
import {
  formatMarketplaceVersion,
  marketplaceEntrySlug,
  marketplaceErrorCode,
  marketplaceErrorMessage,
} from "./marketplace-ui";

export function installedName(entry: MarketplaceCatalogListing): string {
  if (!entry.installed_name) {
    throw new Error(`Installed identity is unavailable for ${entry.name}`);
  }
  return entry.installed_name;
}

/** Use the listed source and digest for this catalog acquisition. */
export function catalogInstallRequest(
  entry: MarketplaceCatalogListing,
  allowUnverified: boolean,
  destination: ReturnType<typeof useExtensionInstanceScope>
): ExtensionInstallRequest {
  return {
    profile: destination.profileName,
    scope: destination.workspaceId ? "workspace" : "global",
    ...(destination.workspaceId ? { workspace_id: destination.workspaceId } : {}),
    allow_unverified: allowUnverified,
    enable: true,
    expected_digest: entry.digest_sha256,
    ref: marketplaceEntrySlug(entry),
    source: entry.source_ref === "catalog:compozy" ? "curated" : "marketplace",
    version: entry.version,
  };
}

/** The destination an install request names, as the enable-failure cache scopes it. */
export function installRequestScope(request: ExtensionInstallRequest) {
  return { profileName: request.profile, workspaceId: request.workspace_id ?? null };
}

export function updatedToast(name: string, version: string | null | undefined) {
  const display = formatMarketplaceVersion(version);
  toast.success(display ? `${name} updated to ${display}` : `${name} updated`);
}

export function reportFailure(error: unknown, fallback: string) {
  const message = marketplaceErrorMessage(error, fallback);
  const code = marketplaceErrorCode(error);
  if (code) toast.error(message, { description: code });
  else toast.error(message);
}
