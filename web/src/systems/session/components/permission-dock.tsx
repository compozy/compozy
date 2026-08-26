import { ChevronUp } from "lucide-react";

import {
  Button,
  cn,
  Dock,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from "@compozy/ui";

// Narrow entry: the dock must not pull the emulator into every session bundle.
import { TerminalApprovalDetail, terminalPermissionDetail } from "@/systems/terminal/parts";
import type { TerminalPermissionDetail } from "@/systems/terminal/parts";

import { usePermissionDock } from "../hooks/use-permission-dock";
import { useSession } from "../hooks/use-sessions";
import type { PermissionRequest } from "../types";

const DANGER_GHOST_CLASS = "text-muted hover:bg-danger-tint hover:text-danger";

/**
 * The plain-language ask, per the locked terminal copy contract.
 *
 * The board title is "«Agent» wants to run · dev server"; the runtime does not
 * carry the terminal's display title here, so the sentence stops at the verb
 * and the detail block beneath states the command, folder, and terminal id.
 */
function terminalAskTitle(detail: TerminalPermissionDetail, agentName: string): string {
  if (detail.kind === "typing") return `${agentName} wants to type`;
  if (detail.kind === "open") return `${agentName} wants to open a terminal`;
  return `${agentName} wants to run`;
}

export interface PermissionDockProps {
  enabled?: boolean;
  permission: PermissionRequest;
  sessionId: string;
  workspaceId: string;
  /** "1/N" when more decisions wait behind this one. */
  countLabel: string | null;
  onResolved: () => void;
}

/**
 * The pending permission as a composer-docked decision panel. Buttons render
 * only for the decisions the runtime offers; reject-always lives behind the
 * reject split menu; digit keys 1–4 decide directly (ignoring focused inputs).
 */
export function PermissionDock({
  enabled = true,
  permission,
  sessionId,
  workspaceId,
  countLabel,
  onResolved,
}: PermissionDockProps) {
  // The runtime classification governs both the visible actions and keyboard
  // shortcuts. A hidden remembered decision must not remain reachable by key.
  const terminalDetail = terminalPermissionDetail(
    permission.toolId ?? permission.toolName,
    permission.toolInput
  );
  const blockedDecisions =
    terminalDetail?.kind === "exec" && terminalDetail.risk !== "ordinary"
      ? (["allow-always"] as const)
      : [];
  const { decide, decisionOptions, isResolved, isSubmitting, subject } = usePermissionDock({
    enabled,
    permission,
    sessionId,
    workspaceId,
    onResolved,
    blockedDecisions,
  });
  // The session names the asking agent; the plain-language title needs it. The
  // detail is already in the query cache for any open session surface.
  const session = useSession(terminalDetail ? sessionId : "");
  const agentName = session.data?.agent_name?.trim() || "The agent";

  if (isResolved) {
    return null;
  }

  const offersRejectOnce = decisionOptions.includes("reject-once");
  // A terminal ask carries facts the generic subject line cannot show: the exact
  // command, where it would run, and why the runtime is asking.
  const offersRejectAlways =
    decisionOptions.includes("reject-always") && terminalDetail?.kind !== "typing";
  // Terminal asks read in the plain register — the raw tool id never leads.
  const irreversible = terminalDetail?.kind === "exec" && terminalDetail.risk === "irreversible";

  return (
    <Dock data-testid="permission-dock" role="region" aria-label="Permission required">
      <Dock.Head>
        <Dock.Eyebrow data-testid="permission-dock-eyebrow">Permission</Dock.Eyebrow>
        <Dock.Title data-testid="permission-dock-title">
          {terminalDetail ? terminalAskTitle(terminalDetail, agentName) : permission.toolName}
        </Dock.Title>
        {countLabel ? (
          <Dock.Count data-testid="permission-dock-count">{countLabel}</Dock.Count>
        ) : null}
      </Dock.Head>
      <Dock.Body>
        {terminalDetail ? (
          <TerminalApprovalDetail detail={terminalDetail} />
        ) : subject ? (
          <Dock.Pre data-testid="permission-dock-subject">{subject}</Dock.Pre>
        ) : null}
        {permission.action || permission.resource ? (
          <Dock.Meta data-testid="permission-dock-meta">
            {permission.action}
            {permission.action && permission.resource ? " · " : ""}
            {permission.resource ? <code>{permission.resource}</code> : null}
          </Dock.Meta>
        ) : null}
      </Dock.Body>
      <Dock.Actions>
        {decisionOptions.includes("allow-once") ? (
          <Button
            size="sm"
            variant={irreversible ? "destructive" : "primary"}
            disabled={isSubmitting}
            onClick={() => decide("allow-once")}
            data-testid="permission-allow-once"
          >
            Allow once
            <Dock.Key>1</Dock.Key>
          </Button>
        ) : null}
        {/* No remembered decision covers the fixed irreversible set (US-022),
            so the option is absent here rather than offered and refused. */}
        {decisionOptions.includes("allow-always") ? (
          <Button
            size="sm"
            variant="outline"
            disabled={isSubmitting}
            onClick={() => decide("allow-always")}
            data-testid="permission-allow-always"
          >
            {terminalDetail?.kind === "typing"
              ? "Allow for this terminal"
              : terminalDetail?.kind === "exec"
                ? "Always allow commands like this"
                : "Always allow"}
            <Dock.Key>2</Dock.Key>
          </Button>
        ) : null}
        <span className="flex-1" />
        {offersRejectOnce ? (
          <div className="inline-flex gap-px">
            <Button
              size="sm"
              variant="ghost"
              className={DANGER_GHOST_CLASS}
              disabled={isSubmitting}
              onClick={() => decide("reject-once")}
              data-testid="permission-reject-once"
            >
              {terminalDetail ? "Don't allow" : "Reject"}
              <Dock.Key>3</Dock.Key>
            </Button>
            {offersRejectAlways ? (
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={
                    <Button
                      size="sm"
                      variant="ghost"
                      className={cn(DANGER_GHOST_CLASS, "group/reject-menu px-1")}
                      disabled={isSubmitting}
                      aria-label="More decline options"
                      data-testid="permission-reject-menu-trigger"
                    />
                  }
                >
                  <ChevronUp
                    aria-hidden="true"
                    className="size-3 transition-transform duration-slow ease-out group-aria-expanded/reject-menu:rotate-180 motion-reduce:transition-none"
                  />
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" data-testid="permission-reject-menu" side="top">
                  <DropdownMenuItem
                    data-testid="permission-reject-always"
                    disabled={isSubmitting}
                    onClick={() => decide("reject-always")}
                    variant="destructive"
                  >
                    {terminalDetail ? "Never allow" : "Reject always"}
                    <DropdownMenuShortcut>4</DropdownMenuShortcut>
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            ) : null}
          </div>
        ) : offersRejectAlways ? (
          <Button
            size="sm"
            variant="ghost"
            className={DANGER_GHOST_CLASS}
            disabled={isSubmitting}
            onClick={() => decide("reject-always")}
            data-testid="permission-reject-always"
          >
            {terminalDetail ? "Never allow" : "Reject always"}
            <Dock.Key>4</Dock.Key>
          </Button>
        ) : null}
      </Dock.Actions>
      {isSubmitting ? (
        <Dock.Status role="status" data-testid="permission-dock-status">
          Submitting decision…
        </Dock.Status>
      ) : null}
    </Dock>
  );
}
