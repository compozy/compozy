import { useIsMutating, useMutationState, type Mutation } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { settingsKeys } from "../lib/query-keys";
import {
  settingsProductUpdate,
  settingsUpdateIndicatorState,
  type ProductUpdateIndicatorState,
} from "../lib/update-product";
import type {
  SettingsUpdateApplyRequest,
  SettingsUpdateApplyResult,
  SettingsUpdateStatus,
} from "../types";
import { useSettingsUpdate } from "./use-settings-sections";
import { useApplySettingsUpdate } from "./use-settings-update-mutations";

type ApplyMutation = Mutation<SettingsUpdateApplyResult, Error, SettingsUpdateApplyRequest>;

interface LatestApply {
  result: SettingsUpdateApplyResult | null;
  error: string | null;
}

/**
 * The one-click product update: the daemon snapshot, the single apply action,
 * and the latest apply outcome shared by every surface that offers it.
 */
export function useProductUpdate() {
  const query = useSettingsUpdate();
  const apply = useApplySettingsUpdate();
  const applying = useIsMutating({ mutationKey: settingsKeys.updateApply() }) > 0;
  const latest = useMutationState({
    filters: { mutationKey: settingsKeys.updateApply(), exact: true },
    select: (mutation: ApplyMutation): LatestApply => ({
      result: mutation.state.data ?? null,
      error: mutation.state.error?.message ?? null,
    }),
  }).at(-1);
  const product = query.data ? settingsProductUpdate(query.data) : null;
  const applyError = applying ? null : (latest?.error ?? null);
  const offered = settingsUpdateIndicatorState(query.data, applying);
  // A refused request leaves the offer standing; say so instead of re-offering
  // the same click as if nothing happened.
  const indicator: ProductUpdateIndicatorState =
    applyError !== null && offered === "available" ? "failed" : offered;

  return {
    query,
    product,
    applying,
    applyResult: latest?.result ?? null,
    applyError,
    indicator,
    /** Applies every eligible part of the product in one durable operation. */
    apply: () => {
      const targets = product?.targets;
      if (targets && !applying) apply.mutate({ targets });
    },
  };
}

export type ProductUpdateModel = ReturnType<typeof useProductUpdate>;

/**
 * Reloads the page once the daemon settles on a different version than the one
 * this page first saw.
 *
 * A runtime update restarts the daemon, which also serves this web bundle, so the
 * running page is stale the moment the new version answers. In the desktop app
 * the installer relaunches the shell before the operation clears, so this only
 * fires in a browser — the "it just comes back updated" half of one-click update.
 */
export function useReloadOnProductVersionChange(snapshot: SettingsUpdateStatus | undefined) {
  const version = snapshot?.runtime.current_version ?? null;
  const [loadedVersion, setLoadedVersion] = useState<string | null>(null);
  if (loadedVersion === null && version !== null) setLoadedVersion(version);
  const settled = snapshot ? !settingsProductUpdate(snapshot).updating : false;
  const stale = settled && loadedVersion !== null && version !== null && version !== loadedVersion;

  useEffect(() => {
    if (stale) window.location.reload();
  }, [stale]);
}
