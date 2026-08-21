/**
 * Usage reporting for client-executed commands.
 *
 * The daemon records its own executions inline; a client operation never
 * reaches it, so the client reports the pair itself. Reporting is
 * fire-and-forget by contract — a failed report must never block or fail the
 * effect the operator already saw land (Key Decisions).
 *
 * `POST /api/cmd-palette/usage` is served by the personalization slice, which
 * also owns the store that gives the record meaning. Until it exists this port
 * is the seam's no-op: nothing is sent, nothing is faked, and the call site does
 * not change when the route lands.
 */
export type CmdPaletteUsageReporter = (commandId: string, query: string) => void;

export const noopUsageReporter: CmdPaletteUsageReporter = () => {};
