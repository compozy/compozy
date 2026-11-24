import { queryOptions } from "@tanstack/react-query";

import {
  fetchAvailableComputers,
  fetchComputer,
  fetchComputers,
  fetchComputerSSHHosts,
} from "../adapters/computers-api";
import { fetchRemoteDevices, fetchRemoteStatus } from "../adapters/remote-host-api";
import { computerRegistryKeys } from "./query-keys";

export function computersListOptions(enabled = true) {
  return queryOptions({
    queryKey: computerRegistryKeys.list(),
    queryFn: ({ signal }) => fetchComputers(signal),
    enabled,
    staleTime: 15_000,
    retry: false,
  });
}

export function computerDetailOptions(id: string, enabled = true) {
  return queryOptions({
    queryKey: computerRegistryKeys.detail(id),
    queryFn: ({ signal }) => fetchComputer(id, signal),
    enabled: enabled && id.trim() !== "",
    staleTime: 15_000,
    retry: false,
  });
}

export function availableComputersOptions(enabled = true) {
  return queryOptions({
    queryKey: computerRegistryKeys.available(),
    queryFn: ({ signal }) => fetchAvailableComputers(signal),
    enabled,
    staleTime: 15_000,
    retry: false,
  });
}

export function computerSSHHostsOptions(enabled = true) {
  return queryOptions({
    queryKey: computerRegistryKeys.sshHosts(),
    queryFn: ({ signal }) => fetchComputerSSHHosts(signal),
    enabled,
    staleTime: 30_000,
    retry: false,
  });
}

export function remoteStatusOptions(enabled = true) {
  return queryOptions({
    queryKey: computerRegistryKeys.remoteStatus(),
    queryFn: ({ signal }) => fetchRemoteStatus(signal),
    enabled,
    staleTime: 15_000,
    retry: false,
  });
}

export function remoteDevicesOptions(enabled = true) {
  return queryOptions({
    queryKey: computerRegistryKeys.remoteDevices(),
    queryFn: ({ signal }) => fetchRemoteDevices(signal),
    enabled,
    staleTime: 15_000,
    retry: false,
  });
}
