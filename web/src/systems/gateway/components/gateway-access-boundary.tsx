import { useLayoutEffect, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { useGatewayAccessState, useGatewayAccessTier } from "../hooks/use-gateway-access";
import { GatewayAccessEnded } from "./gateway-access-ended";
import { GatewayPairingGate } from "./gateway-pairing-gate";

export interface GatewayAccessBoundaryProps {
  children: ReactNode;
  /** Injected in tests so redeeming does not navigate. */
  reload?: () => void;
}

/**
 * Decides whether this browser gets the product or the gate.
 *
 * On a local session the daemon never answers with an access decision, so this
 * is inert and the app renders exactly as before. On a remote session an
 * explicit `gateway_device_unauthenticated` shows the pairing gate and an
 * explicit `gateway_device_revoked` shows the access-ended state.
 *
 * The protected cache is dropped in a layout effect, which React runs after the
 * app subtree has already been unmounted by this same render and before the
 * browser paints — so nothing residual is ever on screen while the drop runs.
 *
 * Removal is chained onto cancellation so every in-flight protected request is
 * settled before anything is dropped, instead of relying on removal to orphan
 * whatever lands late. Removing after the unmount also means no active observer
 * is left to refetch what was dropped.
 */
export function GatewayAccessBoundary({ children, reload }: GatewayAccessBoundaryProps) {
  const access = useGatewayAccessState();
  const tier = useGatewayAccessTier();
  const queryClient = useQueryClient();
  const blocked = access !== "ok";

  useLayoutEffect(() => {
    if (!blocked) return;
    void queryClient.cancelQueries().then(() => queryClient.removeQueries());
  }, [blocked, queryClient]);

  if (access === "revoked") return <GatewayAccessEnded />;
  if (access === "unauthenticated") return <GatewayPairingGate reload={reload} tier={tier} />;
  return children;
}
