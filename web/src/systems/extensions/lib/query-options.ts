import { queryOptions } from "@tanstack/react-query";

import {
  getBundleActivation,
  getExtensionProvenance,
  listBundleActivations,
  listExtensions,
} from "../adapters/extensions-api";
import { extensionKeys } from "./query-keys";

const INVENTORY_STALE_TIME = 30_000;

export const extensionsListOptions = () =>
  queryOptions({
    queryKey: extensionKeys.list(),
    queryFn: ({ signal }) => listExtensions(signal),
    staleTime: INVENTORY_STALE_TIME,
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
