import { queryOptions } from "@tanstack/react-query";

import {
  getBundleActivation,
  getExtensionProvenance,
  listBundleActivations,
  listExtensionLogs,
  listExtensions,
} from "../adapters/extensions-api";
import type { ExtensionInstanceScope } from "../types";
import { extensionKeys } from "./query-keys";

const INVENTORY_STALE_TIME = 30_000;

export const extensionsListOptions = (scope: ExtensionInstanceScope = {}) =>
  queryOptions({
    queryKey: extensionKeys.list(scope.workspaceId),
    queryFn: ({ signal }) => listExtensions(scope, signal),
    staleTime: INVENTORY_STALE_TIME,
  });

export const extensionLogsOptions = (name: string, scope: ExtensionInstanceScope = {}) =>
  queryOptions({
    queryKey: extensionKeys.logs(name, scope.workspaceId),
    queryFn: ({ signal }) => listExtensionLogs(name, scope, signal),
    enabled: name.length > 0,
    staleTime: 0,
  });

export const extensionProvenanceOptions = (name: string, enabled = true) =>
  queryOptions({
    queryKey: extensionKeys.provenance(name),
    queryFn: ({ signal }) => getExtensionProvenance(name, signal),
    enabled: enabled && name.length > 0,
    staleTime: INVENTORY_STALE_TIME,
  });

export const bundleActivationsOptions = () =>
  queryOptions({
    queryKey: extensionKeys.bundles(),
    queryFn: ({ signal }) => listBundleActivations(signal),
    staleTime: INVENTORY_STALE_TIME,
  });

export const bundleActivationOptions = (id: string) =>
  queryOptions({
    queryKey: extensionKeys.bundle(id),
    queryFn: ({ signal }) => getBundleActivation(id, signal),
    enabled: id.length > 0,
    staleTime: INVENTORY_STALE_TIME,
  });
