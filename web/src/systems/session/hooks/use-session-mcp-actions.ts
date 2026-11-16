import { useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";

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

import type {
  SessionMCPResponse,
  SessionMCPServerRow,
  SessionMCPServerTarget,
} from "../adapters/session-mcp-api";
import { sessionKeys } from "../lib/query-keys";
import {
  sameSessionMCPServer,
  sessionMCPCheckedRow,
  sessionMCPServerTarget,
  type SessionMCPRowAction,
} from "../lib/session-mcp-view";
import { invalidateSessionMCPSummary, useCheckSessionMCPServer } from "./use-session-mcp-check";

const DEFINITION_CHANGED = "mcp_approval_definition_changed";

interface SessionMCPActionsInput {
  workspaceId: string;
  sessionId: string;
  profile: string | undefined;
  /** The session's MCP read the open panel takes its row from; `null` before it answered. */
  data: SessionMCPResponse | null;
  /** Read the Settings definitions only while a row in view offers Sign in. */
  enabled: boolean;
  /** No definition matched the row: send the operator to Settings instead. */
  onUnresolved: () => void;
}

type SessionMCPPanelKind = "allow" | "check";

/** The panel the operator opened: which server (by name and owner) and beside which control. */
interface OpenPanel {
  kind: SessionMCPPanelKind;
  target: SessionMCPServerTarget;
  anchor: MCPPanelAnchor;
}

/**
 * A row panel open beside the row button that started it: Review (allow) or Not working (check).
 * `row` is the server's live row from the current read, never a copy taken when it opened.
 */
export interface SessionMCPRowPanel extends OpenPanel {
  row: SessionMCPServerRow;
}

type CheckState = "checking" | "still_failing";

/** One key per server: names repeat across owners. */
function serverKey(target: SessionMCPServerTarget): string {
  return JSON.stringify([target.owner, target.name]);
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
 * daemon's answer. Every answer is tied to the server it was asked for, so it never lands on a
 * panel opened for another one since. Sign in acts on the Settings definition the row names, read
 * while a Sign in row is on screen.
 */
export function useSessionMCPActions({
  workspaceId,
  sessionId,
  profile,
  data,
  enabled,
  onUnresolved,
}: SessionMCPActionsInput) {
  const queryClient = useQueryClient();
  const definitions = useSettingsMCPServers(settingsFilter(workspaceId, profile), { enabled });
  // The sign-in polls its own definition while it runs, apart from the definitions read above
  // (which only runs while a Sign in row is on screen); closing the popover cancels it.
  const authorization = useMCPDefinitionAuthorization();
  const allowMutation = useAllowMCPServer();
  const checkMutation = useCheckSessionMCPServer({ workspaceId, sessionId, profile });
  const [allowingServer, setAllowingServer] = useState<SessionMCPServerTarget | null>(null);
  const [opened, setOpened] = useState<OpenPanel | null>(null);
  const [checks, setChecks] = useState<ReadonlyMap<string, CheckState>>(() => new Map());
  // Checks in flight, read live: a second press in the same tick must not send another.
  const checksInFlight = useRef(new Set<string>());

  const liveRow = opened && data ? sessionMCPCheckedRow(data, opened.target) : undefined;
  // The server left the session's read: its panel closes with it instead of showing a stale row.
  if (opened && data && !liveRow) setOpened(null);
  const panel: SessionMCPRowPanel | null = opened && liveRow ? { ...opened, row: liveRow } : null;

  const setCheck = (target: SessionMCPServerTarget, next: CheckState | null) =>
    setChecks(current => {
      const key = serverKey(target);
      if ((current.get(key) ?? null) === next) return current;
      const updated = new Map(current);
      if (next === null) updated.delete(key);
      else updated.set(key, next);
      return updated;
    });

  const closePanelFor = (target: SessionMCPServerTarget) => {
    setOpened(current =>
      current && sameSessionMCPServer(current.target, target) ? null : current
    );
    setChecks(current => {
      const key = serverKey(target);
      if (current.get(key) !== "still_failing") return current;
      const updated = new Map(current);
      updated.delete(key);
      return updated;
    });
  };

  const closePanel = () => {
    if (opened) closePanelFor(opened.target);
  };

  const rereadSession = () =>
    Promise.all([
      queryClient.invalidateQueries({
        queryKey: sessionKeys.mcp(workspaceId, sessionId),
        exact: true,
      }),
      invalidateSessionMCPSummary(queryClient, workspaceId, sessionId),
    ]);

  const allow = (row: SessionMCPServerRow) => {
    // One Allow at a time: a second click while one is in flight is ignored, not "unresolved".
    if (allowMutation.isPending) return;
    const body = approvalBody(row);
    if (!body) {
      onUnresolved();
      return;
    }
    const target = sessionMCPServerTarget(row);
    setAllowingServer(target);
    allowMutation.mutate(
      { name: row.name, body },
      {
        onSuccess: () => {
          closePanelFor(target);
          toast.success(`${row.name} allowed. This session can use it from your next message.`);
        },
        onError: error => {
          closePanelFor(target);
          toast.error(
            error instanceof SettingsApiError && error.code === DEFINITION_CHANGED
              ? `${row.name} changed since you looked at it. Check the new definition and allow it again.`
              : error.message
          );
        },
        onSettled: () => {
          setAllowingServer(current =>
            current && sameSessionMCPServer(current, target) ? null : current
          );
          void rereadSession();
        },
      }
    );
  };

  const check = (target: SessionMCPServerTarget) => {
    const key = serverKey(target);
    if (checksInFlight.current.has(key)) return;
    checksInFlight.current.add(key);
    setCheck(target, "checking");
    checkMutation
      .mutateAsync(target)
      .then(
        answer => {
          if (sessionMCPCheckedRow(answer, target)?.state === "failed") {
            setCheck(target, "still_failing");
            return;
          }
          setCheck(target, null);
          closePanelFor(target);
        },
        (error: Error) => {
          setCheck(target, null);
          toast.error(error.message);
        }
      )
      .finally(() => checksInFlight.current.delete(key));
  };

  const signIn = (row: SessionMCPServerRow, trigger: HTMLElement) => {
    if (definitions.isPending) return;
    const entry = definitions.data?.mcp_servers.find(item => sameDefinition(item, row));
    const filter = entry ? deriveMCPAuthFilter(entry) : null;
    if (!entry || !filter) {
      onUnresolved();
      return;
    }
    authorization.authorize.requestAuthorize(filter, entry, trigger);
  };

  const act = (row: SessionMCPServerRow, action: SessionMCPRowAction, trigger: HTMLElement) => {
    if (action === "sign_in") {
      signIn(row, trigger);
      return;
    }
    const target = sessionMCPServerTarget(row);
    // A panel opens fresh: an earlier "still not working" does not carry over.
    if (checks.get(serverKey(target)) === "still_failing") setCheck(target, null);
    setOpened({ kind: action, target, anchor: mcpPanelAnchor(trigger) });
  };

  const authorize = authorization.authorize;
  const signingIn: SessionMCPServerTarget | null =
    authorize.phase !== "idle" && authorize.server
      ? { name: authorize.server, owner: authorize.filter?.owner ?? "" }
      : null;
  const panelCheck = panel ? checks.get(serverKey(panel.target)) : undefined;

  return {
    act,
    /** The server whose Allow is in flight. */
    actingServer: allowingServer,
    /** The Settings definitions have not answered yet, so Sign in cannot resolve its target. */
    resolving: enabled && definitions.isPending,
    /** An Allow is in flight; every row's action waits for it. */
    allowing: allowMutation.isPending,
    /** The server whose panel (row panel or sign-in) is open beside its row. */
    activeServer: panel?.target ?? signingIn,
    panel,
    /** The open panel's own Allow is in flight. */
    panelAllowing:
      panel !== null &&
      allowingServer !== null &&
      sameSessionMCPServer(allowingServer, panel.target),
    /** A check of the open panel's server is running. */
    panelChecking: panelCheck === "checking",
    /** The open panel's server answered its last check and still is not working. */
    panelStillFailing: panelCheck === "still_failing",
    closePanel,
    allow,
    check,
    authorization,
  };
}

export type SessionMCPActions = ReturnType<typeof useSessionMCPActions>;
