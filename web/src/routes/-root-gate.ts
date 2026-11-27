import type { QueryClient } from "@tanstack/react-query";
import { redirect } from "@tanstack/react-router";

import { isHostedOriginCandidate } from "@/lib/hosted-browser-auth";
import {
  accountStatusOptions,
  isAccountGateBlocked,
  sanitizeSignInRedirect,
} from "@/systems/account";

const GATE_BYPASS_PATHS = new Set(["/sign-in", "/design-system"]);

/**
 * Root sign-in gate running before `_app`'s `beforeLoad` (`_uiux.md` U0, UT-120).
 *
 * - Hosted origins (`*.compozy.run`) are owned by U8 and never read `/api/account`.
 * - Dev builds (`gate: "off"`) and grace windows (`gate: "grace"`) let `_app` load normally.
 * - Unreachable daemons fall through to the existing boot error screen.
 * - Enforced signed-out / reauth installations redirect to `/sign-in?redirect=<href>`
 *   before any gated profile, workspace, or session read can fire.
 */
export async function guardRootSignInGate(
  queryClient: QueryClient,
  location: { pathname: string; href: string }
): Promise<void> {
  if (isHostedOriginCandidate()) return;
  if (GATE_BYPASS_PATHS.has(location.pathname)) return;

  const status = await queryClient.ensureQueryData(accountStatusOptions()).catch(() => undefined);
  if (!isAccountGateBlocked(status)) return;

  const target = sanitizeSignInRedirect(location.href);
  throw redirect({
    to: "/sign-in",
    search: target ? { redirect: target } : {},
  });
}
