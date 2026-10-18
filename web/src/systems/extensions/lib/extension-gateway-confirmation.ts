import { ExtensionsApiError } from "../adapters/extensions-api";

export const EXTENSION_GATEWAY_CONFIRMATION_CODE = "extension_gateway_confirmation_required";

/**
 * The daemon refuses a lifecycle mutation that would change gateway control permissions until the
 * caller ratifies the exact digest it returns. A boolean cannot ratify a digest the operator never
 * saw, so the affordance is only offered when the daemon actually named one.
 */
export function extensionGatewayConfirmation(error: unknown): { digest: string } | null {
  if (!(error instanceof ExtensionsApiError)) return null;
  if (error.status !== 409) return null;
  if (error.code !== EXTENSION_GATEWAY_CONFIRMATION_CODE) return null;
  const digest = error.currentDigest?.trim();
  return digest ? { digest } : null;
}
