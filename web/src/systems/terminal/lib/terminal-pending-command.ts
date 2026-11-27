import { normalizeComputerId, type ComputerId } from "@/lib/computer-scope";

/**
 * One-shot initial commands queued for freshly opened terminals (for example
 * `claude login` launched from the Providers tab on a remote computer).
 *
 * Consumed on the first live writable attachment so reconnects and window
 * remounts never re-run the command.
 */
const pendingCommands = new Map<string, string>();

function pendingKey(computerId: ComputerId | undefined, terminalId: string): string {
  return `${normalizeComputerId(computerId)}\0${terminalId}`;
}

export function holdPendingTerminalCommand(
  computerId: ComputerId | undefined,
  terminalId: string,
  command: string
): void {
  const trimmed = command.trim();
  if (terminalId.trim() === "" || trimmed === "") return;
  pendingCommands.set(pendingKey(computerId, terminalId), trimmed);
}

export function takePendingTerminalCommand(
  computerId: ComputerId | undefined,
  terminalId: string
): string | null {
  const key = pendingKey(computerId, terminalId);
  const command = pendingCommands.get(key) ?? null;
  if (command !== null) pendingCommands.delete(key);
  return command;
}

export function clearPendingTerminalCommand(
  computerId: ComputerId | undefined,
  terminalId: string
): void {
  pendingCommands.delete(pendingKey(computerId, terminalId));
}
