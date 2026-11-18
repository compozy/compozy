const GENERIC_REDEEM_FAILURE = "Couldn't pair this device. Try again in a moment.";

/**
 * Operator-facing copy for a failed redeem, keyed by the daemon's stable code.
 *
 * An unknown code and a wrong one answer identically on purpose, so the 401
 * copy must not hint which one it was. Internal detail never reaches the page.
 */
export function pairingRedeemFailureMessage(code: string | undefined): string {
  switch (code) {
    case "gateway_invalid_request":
      return "Check the code and the device name, then try again.";
    case "gateway_pairing_invalid":
      return "That code isn't valid. Check it, or create a new code on the machine running CompozyOS.";
    case "gateway_pairing_forbidden":
      return "Pairing isn't allowed from this address. Open the private address or the machine running CompozyOS.";
    case "gateway_pairing_spent":
      return "That code was already used. Create a new code and try again.";
    case "gateway_pairing_expired":
      return "That code has expired. Create a new code and try again.";
    case "gateway_auth_rate_limited":
      return "Too many attempts. Wait a few minutes before trying again.";
    default:
      return GENERIC_REDEEM_FAILURE;
  }
}
