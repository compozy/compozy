/**
 * Which connection-pass failures end the stream instead of retrying.
 *
 * A mint or attach refused because the terminal is gone will be refused on
 * every later attempt too — backing off against it is a spinner over a fact.
 * Everything else (network, expired tickets, daemon restarts) stays retryable.
 */

import { isRemoteRejectionCode } from "@/lib/remote-error-copy";

import { TerminalApiError } from "../adapters/terminal-api";
import type { TerminalErrorCode } from "./terminal-contract-schema";

const TERMINAL_GONE_CODES = new Set<TerminalErrorCode>([
  "terminal_exited",
  "terminal_expired",
  "terminal_not_found",
  "terminal_not_interactive",
]);

export function terminalStreamFatalCode(cause: unknown): string | null {
  if (!(cause instanceof TerminalApiError)) return null;
  if (cause.domainCode !== undefined && TERMINAL_GONE_CODES.has(cause.domainCode)) {
    return cause.domainCode;
  }
  if (isRemoteRejectionCode(cause.code)) return cause.code;
  return null;
}
