/** Outbound frames the browser writes after a daemon `client_command`. */
export type WindowManagerClientCommandOutbound =
  | { type: "client_command_ack"; command_id: string }
  | { type: "client_command_result"; command_id: string; result?: unknown; error?: string };

export function writeWindowManagerClientCommandFrame(
  send: (data: string) => void,
  frame: WindowManagerClientCommandOutbound
): void {
  send(JSON.stringify(frame));
}
