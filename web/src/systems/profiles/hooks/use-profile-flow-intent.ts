import { useEffect, useEffectEvent } from "react";

import { openProfileDialog } from "../stores/profile-dialog-store";
import type { ProfileFlowSearch } from "../lib/profile-flow-search";
import type { ProfileDialogIntent, ProfileLifecycleFlow } from "../types";
import { useGatewayAccessTier } from "@/systems/gateway";

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
 * shell already navigated into the dialog store. An unknown flow is ignored
 * rather than guessed.
 */
export function useProfileFlowIntent(intent: ProfileFlowSearch | undefined): void {
  const tier = useGatewayAccessTier();
  const raise = useEffectEvent((flow: ProfileLifecycleFlow, profile?: string) => {
    const target = profile?.trim() ?? "";
    if (flow === "create") {
      openProfileDialog({ flow, ...(target !== "" ? { profile: target } : {}) });
      return;
    }
    if (target === "") return;
    openProfileDialog({ flow, profile: target } satisfies ProfileDialogIntent);
  });
  const flow = intent?.flow ?? "";
  const profile = intent?.profile ?? "";

  useEffect(() => {
    const resolved = asFlow(flow);
    if (resolved === null || tier !== "local") return;
    raise(resolved, profile === "" ? undefined : profile);
  }, [flow, profile, tier]);
}
