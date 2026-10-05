import { useEffect, useEffectEvent } from "react";

import { useOsShell } from "../../hooks/use-os-shell";
import type { OsWindowRoute } from "../../lib/os-types";
import { useGatewayAccessTier } from "@/systems/gateway";
import {
  openProfileDialog,
  profileFlowFromSearch,
  type ProfileDialogIntent,
  type ProfileLifecycleFlow,
} from "@/systems/profiles";

const FLOWS: readonly ProfileLifecycleFlow[] = [
  "create",
  "update",
  "rename",
  "archive",
  "unarchive",
  "delete",
];

function asFlow(value: string): ProfileLifecycleFlow | null {
  return FLOWS.find(flow => flow === value) ?? null;
}

/**
 * Raises the dialog a palette command navigated here to open.
 *
 * The intent arrives on the window route, which is an external system, so this
 * is one of the narrow cases an effect is the right tool: it syncs a URL the
 * shell already navigated into the dialog store. Its owning window consumes
 * the intent so a later reload cannot replay a canceled action. An unknown
 * flow is ignored rather than guessed.
 */
export function useProfileFlowIntent(windowId: string, route: OsWindowRoute | undefined): void {
  const { manager } = useOsShell();
  const tier = useGatewayAccessTier();
  const intent = route === undefined ? undefined : profileFlowFromSearch(route.search);
  const raise = useEffectEvent((flow: ProfileLifecycleFlow) => {
    if (route === undefined) return;
    const target = intent?.profile ?? "";
    if (flow === "create") {
      const name = intent?.name ?? target;
      openProfileDialog({ flow, ...(name !== "" ? { profile: name } : {}) });
    } else {
      if (target === "") return;
      openProfileDialog({
        flow,
        profile: target,
        ...(flow === "rename" && intent?.new_name !== undefined
          ? { newName: intent.new_name }
          : {}),
      } satisfies ProfileDialogIntent);
    }
    const search = { ...route.search };
    delete search.flow;
    delete search.profile;
    delete search.name;
    delete search.new_name;
    manager.navigateWindow(windowId, { ...route, search });
  });
  const flow = intent?.flow ?? "";
  const profile = intent?.profile ?? "";
  const name = intent?.name ?? "";
  const newName = intent?.new_name ?? "";

  useEffect(() => {
    const resolved = asFlow(flow);
    if (resolved === null || tier !== "local") return;
    raise(resolved);
  }, [flow, profile, name, newName, tier]);
}
