import type { WindowManagerClientCommand } from "./window-manager-types";

export type WindowManagerClientCommandReply = (result?: unknown) => void;
export type WindowManagerClientCommandExecutor = (
  command: WindowManagerClientCommand,
  reply: WindowManagerClientCommandReply
) => unknown | Promise<unknown>;

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

/** Return a terminal result once, including before a command replaces its own connection. */
export function dispatchWindowManagerClientCommand(
  command: WindowManagerClientCommand,
  execute: WindowManagerClientCommandExecutor,
  send: (frame: WindowManagerClientCommandOutbound) => void,
  isActive: () => boolean
): void {
  let replied = false;
  const reply: WindowManagerClientCommandReply = result => {
    if (replied || !isActive()) return;
    replied = true;
    send({
      type: "client_command_result",
      command_id: command.commandId,
      ...(result === undefined ? {} : { result }),
    });
  };
  send({ type: "client_command_ack", command_id: command.commandId });
  void Promise.resolve()
    .then(() => execute(command, reply))
    .then(reply)
    .catch(cause => {
      if (replied || !isActive()) return;
      replied = true;
      send({
        type: "client_command_result",
        command_id: command.commandId,
        error: cause instanceof Error ? cause.message : "The client operation failed.",
      });
    });
}
