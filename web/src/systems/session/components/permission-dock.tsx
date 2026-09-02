import { ChevronUp } from "lucide-react";

import {
  Button,
  Dock,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from "@compozy/ui";

import {
  TerminalApprovalDetail,
  terminalAlwaysAllowLabel,
  terminalAskTitle,
  terminalBlockedRememberedDecisions,
  terminalIdFromDetail,
  terminalPermissionDetail,
  terminalRejectOnceLabel,
  useKnownTerminalTitle,
} from "@/systems/terminal/parts";

import { usePermissionDock } from "../hooks/use-permission-dock";
import { useSession } from "../hooks/use-sessions";
import type { PermissionDecision } from "../adapters/session-api";
import type { PermissionRequest } from "../types";

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
 * reject split menu on ordinary asks; digit keys 1–4 decide directly (ignoring
 * focused inputs). Exec that always asks withholds both remembered polarities.
 *
 * Host chrome stays Dock — the live decision surface. Terminal facts flavor
 * the body as an attention-row read inside that host.
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
  const blockedDecisions = terminalBlockedRememberedDecisions(terminalDetail);
  const { decide, decisionOptions, isResolved, isSubmitting, subject } = usePermissionDock({
    enabled,
    permission,
    sessionId,
    workspaceId,
    onResolved,
    blockedDecisions,
  });
  const session = useSession(terminalDetail ? sessionId : "");
  const agentName = session.data?.agent_name?.trim() || "The agent";
  const catalogTitle = useKnownTerminalTitle(
    workspaceId,
    session.data?.profile_name,
    terminalDetail ? terminalIdFromDetail(terminalDetail) : null
  );

  if (isResolved) {
    return null;
  }

  const irreversible = terminalDetail?.kind === "exec" && terminalDetail.risk === "irreversible";

  return (
    <Dock data-testid="permission-dock" role="region" aria-label="Permission required">
      <PermissionDockHead
        agentName={agentName}
        catalogTitle={catalogTitle}
        countLabel={countLabel}
        permission={permission}
        terminalDetail={terminalDetail}
      />
      <PermissionDockBody
        catalogTitle={catalogTitle}
        permission={permission}
        subject={subject}
        terminalDetail={terminalDetail}
      />
      <PermissionDockActions
        decide={decide}
        decisionOptions={decisionOptions}
        irreversible={irreversible}
        isSubmitting={isSubmitting}
        terminalDetail={terminalDetail}
      />
      {isSubmitting ? (
        <Dock.Status role="status" data-testid="permission-dock-status">
          Submitting decision…
        </Dock.Status>
      ) : null}
    </Dock>
  );
}

function PermissionDockHead({
  agentName,
  catalogTitle,
  countLabel,
  permission,
  terminalDetail,
}: Pick<PermissionDockProps, "countLabel" | "permission"> & {
  agentName: string;
  catalogTitle?: string;
  terminalDetail: ReturnType<typeof terminalPermissionDetail>;
}) {
  return (
    <Dock.Head>
      <Dock.Eyebrow data-testid="permission-dock-eyebrow">Permission</Dock.Eyebrow>
      <Dock.Title data-testid="permission-dock-title">
        {terminalDetail
          ? terminalAskTitle(terminalDetail, agentName, catalogTitle)
          : permission.toolName}
      </Dock.Title>
      {countLabel ? (
        <Dock.Count data-testid="permission-dock-count">{countLabel}</Dock.Count>
      ) : null}
    </Dock.Head>
  );
}

function PermissionDockBody({
  catalogTitle,
  permission,
  subject,
  terminalDetail,
}: Pick<PermissionDockProps, "permission"> & {
  catalogTitle?: string;
  subject: string | null;
  terminalDetail: ReturnType<typeof terminalPermissionDetail>;
}) {
  return (
    <Dock.Body>
      {terminalDetail ? (
        <TerminalApprovalDetail detail={terminalDetail} terminalTitle={catalogTitle} />
      ) : subject ? (
        <Dock.Pre data-testid="permission-dock-subject">{subject}</Dock.Pre>
      ) : null}
      {!terminalDetail && (permission.action || permission.resource) ? (
        <Dock.Meta data-testid="permission-dock-meta">
          {permission.action}
          {permission.action && permission.resource ? " · " : ""}
          {permission.resource ? <code>{permission.resource}</code> : null}
        </Dock.Meta>
      ) : null}
    </Dock.Body>
  );
}

function PermissionDockActions({
  decide,
  decisionOptions,
  irreversible,
  isSubmitting,
  terminalDetail,
}: {
  decide: (decision: PermissionDecision) => void;
  decisionOptions: PermissionDecision[];
  irreversible: boolean;
  isSubmitting: boolean;
  terminalDetail: ReturnType<typeof terminalPermissionDetail>;
}) {
  const offersRejectOnce = decisionOptions.includes("reject-once");
  const offersRejectAlways =
    decisionOptions.includes("reject-always") && terminalDetail?.kind !== "typing";

  return (
    <Dock.Actions>
      <PermissionAllowOnceAction
        available={decisionOptions.includes("allow-once")}
        decide={decide}
        irreversible={irreversible}
        isSubmitting={isSubmitting}
      />
      <PermissionAllowAlwaysAction
        available={decisionOptions.includes("allow-always")}
        decide={decide}
        isSubmitting={isSubmitting}
        terminalDetail={terminalDetail}
      />
      <span className="flex-1" />
      <PermissionRejectActions
        decide={decide}
        isSubmitting={isSubmitting}
        offersRejectAlways={offersRejectAlways}
        offersRejectOnce={offersRejectOnce}
        terminalDetail={terminalDetail}
      />
    </Dock.Actions>
  );
}

interface PermissionActionProps {
  decide: (decision: PermissionDecision) => void;
  isSubmitting: boolean;
}

function PermissionAllowOnceAction({
  available,
  decide,
  irreversible,
  isSubmitting,
}: PermissionActionProps & { available: boolean; irreversible: boolean }) {
  if (!available) return null;
  return (
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
  );
}

function PermissionAllowAlwaysAction({
  available,
  decide,
  isSubmitting,
  terminalDetail,
}: PermissionActionProps & {
  available: boolean;
  terminalDetail: ReturnType<typeof terminalPermissionDetail>;
}) {
  if (!available) return null;
  return (
    <Button
      size="sm"
      variant="outline"
      disabled={isSubmitting}
      onClick={() => decide("allow-always")}
      data-testid="permission-allow-always"
    >
      {terminalDetail ? terminalAlwaysAllowLabel(terminalDetail) : "Always allow"}
      <Dock.Key>2</Dock.Key>
    </Button>
  );
}

function PermissionRejectActions({
  decide,
  isSubmitting,
  offersRejectAlways,
  offersRejectOnce,
  terminalDetail,
}: PermissionActionProps & {
  offersRejectAlways: boolean;
  offersRejectOnce: boolean;
  terminalDetail: ReturnType<typeof terminalPermissionDetail>;
}) {
  if (!offersRejectOnce) {
    return offersRejectAlways ? (
      <Button
        size="sm"
        variant="ghost"
        disabled={isSubmitting}
        onClick={() => decide("reject-always")}
        data-testid="permission-reject-always"
      >
        {terminalDetail ? "Never allow" : "Reject always"}
        <Dock.Key>4</Dock.Key>
      </Button>
    ) : null;
  }

  return (
    <div className="inline-flex gap-px">
      <Button
        size="sm"
        variant="ghost"
        disabled={isSubmitting}
        onClick={() => decide("reject-once")}
        data-testid="permission-reject-once"
      >
        {terminalDetail ? terminalRejectOnceLabel() : "Reject"}
        <Dock.Key>3</Dock.Key>
      </Button>
      {offersRejectAlways ? (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                size="sm"
                variant="ghost"
                className="group/reject-menu px-1"
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
  );
}
