/**
 * Classification of the two loopback-guard 403 envelopes into a gateway
 * domain signal.
 *
 * The wire shapes are frozen in the spec's DX contract: a 403 whose JSON body
 * carries `code: "loopback_mutation_required"` or `code:
 * "loopback_api_required"` means the daemon cannot execute the call from this
 * listener and the action belongs on the daemon host. The codes are owned
 * here (the gateway system), not in the transport seam — the transport hands
 * forbidden envelopes over raw and never learns what they mean.
 *
 * Deliberately narrow: only an HTTP 403 carrying one of the two explicit
 * daemon codes classifies. Any other status, a missing code, or an unrelated
 * code returns `undefined` — ordinary authorization refusals are not
 * loopback signals and must not render the loopback-only state.
 */
import { gatewayErrorCode } from "@/lib/gateway-access-signal";

const LOOPBACK_MUTATION_CODE = "loopback_mutation_required";
const LOOPBACK_API_CODE = "loopback_api_required";

/**
 * Which loopback guard refused, mirroring the two wire codes one-to-one:
 * `mutation` ← `loopback_mutation_required`, `api` ← `loopback_api_required`.
 * Both render the same truthful loopback-only state; the distinction records
 * which guard fired.
 */
export type GatewayLoopbackSignal = "mutation" | "api";

/**
 * Returns the loopback signal a response represents, or `undefined` when the
 * failure says nothing about loopback capability.
 */
export function classifyGatewayLoopback(
  status: number,
  payload: unknown
): GatewayLoopbackSignal | undefined {
  if (status !== 403) return undefined;
  switch (gatewayErrorCode(payload)) {
    case LOOPBACK_MUTATION_CODE:
      return "mutation";
    case LOOPBACK_API_CODE:
      return "api";
    default:
      return undefined;
  }
}
