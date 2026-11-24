import { queryOptions } from "@tanstack/react-query";

import {
  fetchAccountApprovals,
  fetchAccountDevices,
  fetchAccountStatus,
} from "../adapters/account-api";
import { accountKeys } from "./query-keys";

export function accountStatusOptions(enabled = true) {
  return queryOptions({
    queryKey: accountKeys.status(),
    queryFn: ({ signal }) => fetchAccountStatus(signal),
    enabled,
    staleTime: 30_000,
    retry: false,
  });
}

export function accountDevicesOptions(enabled = true) {
  return queryOptions({
    queryKey: accountKeys.devices(),
    queryFn: ({ signal }) => fetchAccountDevices(signal),
    enabled,
    staleTime: 15_000,
    retry: false,
  });
}

export function accountApprovalsOptions(enabled = true) {
  return queryOptions({
    queryKey: accountKeys.approvals(),
    queryFn: ({ signal }) => fetchAccountApprovals(signal),
    enabled,
    staleTime: 15_000,
    retry: false,
  });
}
