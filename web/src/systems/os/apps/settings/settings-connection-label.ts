import type { ConnectionStatus } from "@compozy/ui";

const CONNECTION_LABEL: Record<ConnectionStatus, string> = {
  connected: "CompozyOS running",
  connecting: "CompozyOS connecting",
  disconnected: "CompozyOS unreachable",
  error: "CompozyOS unreachable",
};

export function settingsConnectionLabel(connection: ConnectionStatus): string {
  return CONNECTION_LABEL[connection];
}
