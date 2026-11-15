import type { MCPCopyFailure } from "@/systems/settings/lib/mcp-runtime-view-model";
import { mcpManagementServerFixtures } from "@/systems/settings/mocks/fixtures";
import type { RuntimeMCPServer, SettingsMCPServerEntry } from "@/systems/settings/types";

export const COMPOZY_SERVERS: readonly SettingsMCPServerEntry[] = mcpManagementServerFixtures.slice(
  0,
  5
);

export type CopyDialogCase = "default" | "name-taken" | "source-changed" | "submitting";

export function copyFailure(
  which: CopyDialogCase | undefined,
  server: RuntimeMCPServer | undefined
): MCPCopyFailure | null {
  if (!server) return null;
  if (which === "name-taken") {
    return {
      code: "mcp_import_name_taken",
      message: `A CompozyOS server named ${server.name} already exists in Personal`,
    };
  }
  if (which === "source-changed") {
    return {
      code: "mcp_server_not_found",
      message: `The server ${server.name} is no longer in ${server.source_label}`,
    };
  }
  return null;
}
