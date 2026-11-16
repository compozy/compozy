import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast } from "@compozy/ui";
import {
  deriveMCPAuthFilter,
  mcpPanelAnchor,
  SettingsApiError,
  useAllowMCPServer,
  useMCPDefinitionAuthorization,
  useSettingsMCPServers,
  type MCPPanelAnchor,
  type SettingsMCPServerEntry,
  type SettingsMCPServerListFilter,
} from "@/systems/settings";

import type { SessionMCPServerRow, SessionMCPServerTarget } from "../adapters/session-mcp-api";
import { sessionKeys } from "../lib/query-keys";
import type { SessionMCPRowAction } from "../lib/session-mcp-view";
import { sessionMCPCheckedRow, useCheckSessionMCPServer } from "./use-session-mcp-check";

const DEFINITION_CHANGED = "mcp_approval_definition_changed";

interface SessionMCPActionsInput {
  workspaceId: string;
  sessionId: string;
  profile: string | undefined;
  /** Read the Settings definitions only while a row in view offers Sign in. */
  enabled: boolean;
  /** No definition matched the row: send the operator to Settings instead. */
  onUnresolved: () => void;
}

/** A row panel open beside the row button that started it: Review (allow) or Not working (check). */
export interface SessionMCPRowPanel {
  kind: "allow" | "check";
  target: SessionMCPServerTarget;
  anchor: MCPPanelAnchor;
}

/**
 * The Settings lens the session's servers resolve in: its own profile, inside its project. A User
 * session (empty `workspaceId`) resolves in the User lens, with no project.
 */
function settingsFilter(
  workspaceId: string,
  profile: string | undefined
): SettingsMCPServerListFilter {
  const named = profile?.trim();
  const project = workspaceId.trim();
  if (named && named !== "default") {
    return { scope: "profile", profile: named, ...(project ? { workspace_id: project } : {}) };
  }
  return project ? { scope: "workspace", workspace_id: project } : { scope: "user" };
}

function sameDefinition(entry: SettingsMCPServerEntry, row: SessionMCPServerRow): boolean {
  return entry.name === row.name && (entry.owner ?? "") === row.owner;
}

/** The approval request a waiting row carries: its project, profile, and the digest it showed. */
function approvalBody(row: SessionMCPServerRow) {
  const workspaceId = row.workspace_id?.trim();
  const digest = row.approval?.definition_digest.trim();
  if (!workspaceId || !digest) return null;
  const profile = row.profile?.trim();
  return {
    workspace_id: workspaceId,
    ...(profile ? { profile } : {}),
    definition_digest: digest,
  };
}

/**
 * Sign in, Review and Not working from the composer's MCP popover. Review and Not working open a
 * panel beside the row; Allow binds the digest the row showed (a definition that changed in
 * between is refused and the rows reread), and Check again replaces the session's MCP read with the
 * daemon's answer. Sign in acts on the Settings definition the row names, read while a Sign in row
 * is on screen.
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
  // The sign-in outlives a closed popover, so its own definition poll is not tied to it.
  const authorization = useMCPDefinitionAuthorization();
  const allowMutation = useAllowMCPServer();
  const checkMutation = useCheckSessionMCPServer({ workspaceId, sessionId, profile });
  const [actingName, setActingName] = useState<string | null>(null);
  const [panel, setPanel] = useState<SessionMCPRowPanel | null>(null);
  /** The last Check again answered and the server still is not working. */
  const [stillFailing, setStillFailing] = useState(false);

  const rereadSession = () =>
    queryClient.invalidateQueries({ queryKey: sessionKeys.mcp(workspaceId, sessionId) });

  const closePanel = () => {
    setPanel(null);
    setStillFailing(false);
  };

  const allow = (row: SessionMCPServerRow) => {
    // One Allow at a time: a second click while one is in flight is ignored, not "unresolved".
    if (allowMutation.isPending) return;
    const body = approvalBody(row);
    if (!body) {
      onUnresolved();
      return;
    }
    setActingName(row.name);
    allowMutation.mutate(
      { name: row.name, body },
      {
        onSuccess: () => {
          closePanel();
          toast.success(`${row.name} allowed. This session can use it from your next message.`);
        },
        onError: error => {
          closePanel();
          toast.error(
            error instanceof SettingsApiError && error.code === DEFINITION_CHANGED
              ? `${row.name} changed since you looked at it. Check the new definition and allow it again.`
              : error.message
          );
        },
        onSettled: () => {
          setActingName(null);
          void rereadSession();
        },
      }
    );
  };

  const check = (target: SessionMCPServerTarget) => {
    if (checkMutation.isPending) return;
    setStillFailing(false);
    checkMutation.mutate(target, {
      onSuccess: data => {
        if (sessionMCPCheckedRow(data, target)?.state === "failed") setStillFailing(true);
        else closePanel();
      },
      onError: error => toast.error(error.message),
    });
  };

  const signIn = (row: SessionMCPServerRow, trigger?: HTMLElement) => {
    if (definitions.isPending) return;
    const entry = definitions.data?.mcp_servers.find(item => sameDefinition(item, row));
    const filter = entry ? deriveMCPAuthFilter(entry) : null;
    if (!entry || !filter) {
      onUnresolved();
      return;
    }
    authorization.authorize.requestAuthorize(filter, entry, trigger);
  };

  const act = (row: SessionMCPServerRow, action: SessionMCPRowAction, trigger?: HTMLElement) => {
    if (action === "sign_in") {
      signIn(row, trigger);
      return;
    }
    if (!trigger) return;
    setStillFailing(false);
    setPanel({
      kind: action,
      target: { name: row.name, owner: row.owner },
      anchor: mcpPanelAnchor(trigger),
    });
  };

  return {
    act,
    actingName,
    /** The Settings definitions have not answered yet, so Sign in cannot resolve its target. */
    resolving: enabled && definitions.isPending,
    /** An Allow is in flight; every row's action waits for it. */
    allowing: allowMutation.isPending,
    panel,
    closePanel,
    allow,
    check,
    checking: checkMutation.isPending,
    stillFailing,
    authorization,
  };
}

export type SessionMCPActions = ReturnType<typeof useSessionMCPActions>;
