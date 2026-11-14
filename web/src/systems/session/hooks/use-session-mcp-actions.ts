import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast } from "@compozy/ui";
import {
  deriveMCPAuthFilter,
  mcpApprovalTarget,
  SettingsApiError,
  useAllowMCPServer,
  useMCPDefinitionAuthorization,
  useSettingsMCPServers,
  type SettingsMCPServerEntry,
  type SettingsMCPServerListFilter,
} from "@/systems/settings";

import type { SessionMCPServerRow } from "../adapters/session-mcp-api";
import { sessionKeys } from "../lib/query-keys";
import type { SessionMCPRowAction } from "../lib/session-mcp-view";

const DEFINITION_CHANGED = "mcp_approval_definition_changed";

interface SessionMCPActionsInput {
  workspaceId: string;
  sessionId: string;
  profile: string | undefined;
  /** Read the Settings definitions only while a row in view offers an action. */
  enabled: boolean;
  /** No Settings definition matched the row: send the operator to Settings instead. */
  onUnresolved: () => void;
}

/** The Settings lens the session's servers resolve in: its own profile, inside its project. */
function settingsFilter(
  workspaceId: string,
  profile: string | undefined
): SettingsMCPServerListFilter {
  const named = profile?.trim();
  if (named && named !== "default") {
    return { scope: "profile", profile: named, workspace_id: workspaceId };
  }
  return { scope: "workspace", workspace_id: workspaceId };
}

function sameDefinition(entry: SettingsMCPServerEntry, row: SessionMCPServerRow): boolean {
  return entry.name === row.name && (entry.owner ?? "") === row.owner;
}

/**
 * Sign in and Allow from the composer's MCP popover. Both act on the Settings definition the
 * session row names (the row carries no digest or auth filter), so the matching Settings entry is
 * read while an action is on screen. Allow binds the digest that entry showed; a definition that
 * changed in between is refused and the rows reread.
 */
export function useSessionMCPActions({
  workspaceId,
  sessionId,
  profile,
  enabled,
  onUnresolved,
}: SessionMCPActionsInput) {
  const queryClient = useQueryClient();
  const definitions = useSettingsMCPServers(settingsFilter(workspaceId, profile), { enabled });
  // The sign-in dialog outlives the popover, so its own definition poll is not tied to it.
  const authorization = useMCPDefinitionAuthorization();
  const allowMutation = useAllowMCPServer();
  const [actingName, setActingName] = useState<string | null>(null);

  const rereadSession = () =>
    queryClient.invalidateQueries({ queryKey: sessionKeys.mcp(workspaceId, sessionId) });

  const allow = (entry: SettingsMCPServerEntry) => {
    const target = mcpApprovalTarget(entry);
    if (!target || allowMutation.isPending) {
      onUnresolved();
      return;
    }
    setActingName(entry.name);
    allowMutation.mutate(
      {
        name: entry.name,
        body: { ...target, definition_digest: entry.approval.definition_digest },
      },
      {
        onSuccess: () =>
          toast.success(`${entry.name} allowed. This session can use it from your next message.`),
        onError: error =>
          toast.error(
            error instanceof SettingsApiError && error.code === DEFINITION_CHANGED
              ? `${entry.name} changed since you looked at it. Check the new definition and allow it again.`
              : error.message
          ),
        onSettled: () => {
          setActingName(null);
          void rereadSession();
        },
      }
    );
  };

  const act = (row: SessionMCPServerRow, action: SessionMCPRowAction) => {
    if (definitions.isPending) return;
    const entry = definitions.data?.mcp_servers.find(item => sameDefinition(item, row));
    if (!entry) {
      onUnresolved();
      return;
    }
    if (action === "allow") {
      allow(entry);
      return;
    }
    const filter = deriveMCPAuthFilter(entry);
    if (!filter) {
      onUnresolved();
      return;
    }
    authorization.authorize.requestAuthorize(filter, entry);
  };

  return {
    act,
    actingName,
    /** The Settings definitions have not answered yet, so no action can resolve its target. */
    resolving: enabled && definitions.isPending,
    authorization,
  };
}
