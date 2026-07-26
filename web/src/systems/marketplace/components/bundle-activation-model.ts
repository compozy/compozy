import type { BundlePreviewRequest, MarketplaceEntryResponse } from "../types";

export function buildBundleRequest(
  data: MarketplaceEntryResponse,
  profile: string,
  scope: "global" | "workspace",
  workspaceId: string | null | undefined,
  confirmNetworkRequirement = false
): BundlePreviewRequest {
  if (!data.bundle) throw new Error("Bundle detail is required");
  return {
    bundle_name: data.entry.name,
    ...(confirmNetworkRequirement ? { confirm_network_requirement: true } : {}),
    extension_name: data.bundle.extension_name,
    profile_name: profile,
    scope,
    workspace: scope === "workspace" ? (workspaceId ?? undefined) : undefined,
  };
}
