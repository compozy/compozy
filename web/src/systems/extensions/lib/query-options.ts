import { queryOptions } from "@tanstack/react-query";

import {
  getExtensionInventory,
  getExtensionProvenance,
  listExtensionLogs,
  listExtensions,
} from "../adapters/extensions-api";
import type { ExtensionInstanceScope } from "../types";
import { normalizeExtensionLogSnapshot } from "./extension-log-stream";
import { extensionKeys } from "./query-keys";

const INVENTORY_STALE_TIME = 30_000;

export const extensionsListOptions = (scope: ExtensionInstanceScope = {}, enabled = true) =>
  queryOptions({
    queryKey: extensionKeys.list(scope.workspaceId),
    queryFn: ({ signal }) => listExtensions(scope, signal),
    staleTime: INVENTORY_STALE_TIME,
    enabled,
  });

export const extensionLogsOptions = (name: string, scope: ExtensionInstanceScope = {}) => {
  const normalizedName = name.trim();
  const normalizedWorkspaceId = scope.workspaceId?.trim() || undefined;
  return queryOptions({
    queryKey: extensionKeys.logs(normalizedName, normalizedWorkspaceId),
    queryFn: async ({ signal }) =>
      normalizeExtensionLogSnapshot(
        await listExtensionLogs(normalizedName, { workspaceId: normalizedWorkspaceId }, signal)
      ),
    enabled: normalizedName.length > 0,
    staleTime: 0,
  });
};

export const extensionProvenanceOptions = (name: string, enabled = true) =>
  queryOptions({
    queryKey: extensionKeys.provenance(name),
    queryFn: ({ signal }) => getExtensionProvenance(name, signal),
    enabled: enabled && name.length > 0,
    staleTime: INVENTORY_STALE_TIME,
  });

export const extensionInventoryOptions = (name: string, enabled = true) =>
  queryOptions({
    queryKey: extensionKeys.inventory(name),
    queryFn: ({ signal }) => getExtensionInventory(name, signal),
    enabled: enabled && name.length > 0,
    staleTime: INVENTORY_STALE_TIME,
  });
