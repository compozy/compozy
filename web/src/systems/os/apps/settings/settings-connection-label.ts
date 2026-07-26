import type { ConnectionStatus } from "@agh/ui";

const CONNECTION_LABEL: Record<ConnectionStatus, string> = {
  connected: "Daemon running",
  connecting: "Daemon connecting",
  disconnected: "Daemon unreachable",
  error: "Daemon unreachable",
};

export function settingsConnectionLabel(connection: ConnectionStatus): string {
  return CONNECTION_LABEL[connection];
}
