import { apiErrorMessage } from "@/lib/api-client";
import { gatewayErrorCode } from "@/lib/gateway-access-signal";

/**
 * Carries the daemon's stable machine code alongside the status so callers can
 * branch on the decision ("consent required", "generation conflict", "pairing
 * spent") instead of matching message strings.
 */
export class GatewayApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code?: string
  ) {
    super(message);
    this.name = "GatewayApiError";
  }
}

export function gatewayApiError(
  fallback: string,
  response: Response,
  error: unknown
): GatewayApiError {
  const code = gatewayErrorCode(error);
  return new GatewayApiError(
    apiErrorMessage(error) ?? `${fallback}: ${response.status}`,
    response.status,
    code
  );
}
