/**
 * Which connection-pass failures end the stream instead of retrying.
 *
 * A mint or attach refused because the terminal is gone will be refused on
 * every later attempt too — backing off against it is a spinner over a fact.
 * Everything else (network, expired tickets, daemon restarts) stays retryable.
 */

import { TerminalApiError } from "../adapters/terminal-api";
import type { TerminalErrorCode } from "./terminal-contract-schema";

const TERMINAL_GONE_CODES = new Set<TerminalErrorCode>([
  "terminal_exited",
  "terminal_expired",
  "terminal_not_found",
]);

export function terminalStreamFatalCode(cause: unknown): TerminalErrorCode | null {
  if (!(cause instanceof TerminalApiError) || cause.domainCode === undefined) return null;
  return TERMINAL_GONE_CODES.has(cause.domainCode) ? cause.domainCode : null;
}
