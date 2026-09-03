import type { TerminalControlRead } from "./terminal-lease";

/** The design-contract values a lease chip may publish. */
export type TerminalLeaseDataAttr = "me" | "agent" | "available";

/**
 * Maps the lease read onto the chip's data-lease contract.
 *
 * `me` is only the viewer who can type. `available` is nobody — a free
 * terminal, visually neutral. Another human is neither, so the attribute is
 * omitted rather than lying that the seat is free or that an agent holds it.
 */
export function terminalLeaseDataAttr(
  read: TerminalControlRead
): TerminalLeaseDataAttr | undefined {
  if (read === "you") return "me";
  if (read === "agent") return "agent";
  if (read === "nobody") return "available";
  return undefined;
}
