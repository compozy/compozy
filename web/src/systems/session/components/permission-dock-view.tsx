import { ChevronUp, SquareTerminal } from "lucide-react";
import type { ReactNode } from "react";

import {
  Button,
  Dock,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
  KindIcon,
} from "@compozy/ui";

import { TerminalApprovalDetail } from "@/systems/terminal/components/terminal-approval-detail";
import {
  terminalAlwaysAllowLabel,
  terminalAskTitle,
  terminalRejectOnceLabel,
} from "@/systems/terminal/lib/terminal-permission-copy";
import type { terminalPermissionDetail } from "@/systems/terminal/lib/terminal-permission";

import { getToolIcon, resolveRegisteredToolName, toolAskPhrase } from "../lib/tool-labels";
import type { PermissionDecision } from "../adapters/session-api";
import type { PermissionRequest } from "../types";

export interface PermissionDockViewProps {
  agentName: string;
  catalogTitle?: string;
  /** "1/N" when more decisions wait behind this one. */
  countLabel: string | null;
  decide: (decision: PermissionDecision) => void;
  decisionOptions: PermissionDecision[];
  isSubmitting: boolean;
  permission: PermissionRequest;
  subject: string | null;
  terminalDetail: ReturnType<typeof terminalPermissionDetail>;
  /** Where the ask came from, when it reached this session from another one. */
  origin?: ReactNode;
}

/** The dock's anatomy from resolved props; `PermissionDock` binds the session and the decision. */
export function PermissionDockView({
  agentName,
  catalogTitle,
  countLabel,
  decide,
  decisionOptions,
  isSubmitting,
  origin,
  permission,
  subject,
  terminalDetail,
}: PermissionDockViewProps) {
  const irreversible = terminalDetail?.kind === "exec" && terminalDetail.risk === "irreversible";
  return (
    <Dock
      data-testid="permission-dock"
      data-permission-action={permission.action || undefined}
      role="region"
      aria-label="Permission required"
    >
      <PermissionDockHead
        agentName={agentName}
        catalogTitle={catalogTitle}
        countLabel={countLabel}
        permission={permission}
        terminalDetail={terminalDetail}
      />
      <PermissionDockBody
        permission={permission}
        subject={subject}
        terminalDetail={terminalDetail}
      />
      {origin}
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
}: Pick<PermissionDockViewProps, "countLabel" | "permission"> & {
  agentName: string;
  catalogTitle?: string;
  terminalDetail: ReturnType<typeof terminalPermissionDetail>;
}) {
  return (
    <Dock.Head>
      <KindIcon
        data-testid="permission-dock-well"
        icon={
          terminalDetail
            ? SquareTerminal
            : getToolIcon(
                resolveRegisteredToolName(permission.toolId ?? permission.toolName),
                permission.toolInput
              )
        }
        tone="well"
      />
      <Dock.Title data-testid="permission-dock-title" title={permission.toolId ?? undefined}>
        {terminalDetail
          ? terminalAskTitle(terminalDetail, agentName, catalogTitle)
          : genericAskTitle(permission.toolName, agentName)}
      </Dock.Title>
      {countLabel ? (
        <Dock.Count data-testid="permission-dock-count">{countLabel}</Dock.Count>
      ) : null}
    </Dock.Head>
  );
}

/** "Allow Claude to edit file?" for known or raw tool ids; a readable runtime title leads as is. */
function genericAskTitle(toolName: string, agentName: string): string {
  const phrase = toolAskPhrase(toolName);
  if (phrase === null) return toolName;
  const subject = agentName === "The agent" ? "the agent" : agentName;
  return `Allow ${subject} to ${phrase}?`;
}

function PermissionDockBody({
  permission,
  subject,
  terminalDetail,
}: Pick<PermissionDockViewProps, "permission"> & {
  subject: string | null;
  terminalDetail: ReturnType<typeof terminalPermissionDetail>;
}) {
  return (
    <Dock.Body>
      {terminalDetail ? (
        <TerminalApprovalDetail detail={terminalDetail} />
      ) : subject ? (
        <Dock.Pre data-testid="permission-dock-subject">{subject}</Dock.Pre>
      ) : null}
      {/* The protocol action rides on the dock's data-permission-action; only the resource is shown. */}
      {!terminalDetail && permission.resource ? (
        <Dock.Meta data-testid="permission-dock-meta">
          <code>{permission.resource}</code>
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
  const offersRejectAlways = decisionOptions.includes("reject-always");

  return (
    <Dock.Actions>
      <span className="flex-1" />
      <PermissionRejectActions
        decide={decide}
        isSubmitting={isSubmitting}
        offersRejectAlways={offersRejectAlways}
        offersRejectOnce={offersRejectOnce}
      />
      <PermissionAllowAlwaysAction
        available={decisionOptions.includes("allow-always")}
        decide={decide}
        isSubmitting={isSubmitting}
        terminalDetail={terminalDetail}
      />
      <PermissionAllowOnceAction
        available={decisionOptions.includes("allow-once")}
        decide={decide}
        irreversible={irreversible}
        isSubmitting={isSubmitting}
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
      variant={irreversible ? "destructive" : "primary"}
      disabled={isSubmitting}
      onClick={() => decide("allow-once")}
      aria-keyshortcuts="1"
      kbd="1"
      data-testid="permission-allow-once"
    >
      Allow once
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
      variant="secondary"
      disabled={isSubmitting}
      onClick={() => decide("allow-always")}
      aria-keyshortcuts="2"
      kbd="2"
      data-testid="permission-allow-always"
    >
      {terminalDetail ? terminalAlwaysAllowLabel(terminalDetail) : "Always allow"}
    </Button>
  );
}

function PermissionRejectActions({
  decide,
  isSubmitting,
  offersRejectAlways,
  offersRejectOnce,
}: PermissionActionProps & {
  offersRejectAlways: boolean;
  offersRejectOnce: boolean;
}) {
  if (!offersRejectOnce) {
    return offersRejectAlways ? (
      <Button
        variant="secondary"
        disabled={isSubmitting}
        onClick={() => decide("reject-always")}
        aria-keyshortcuts="4"
        kbd="4"
        data-testid="permission-reject-always"
      >
        Never allow
      </Button>
    ) : null;
  }

  return (
    <div className="inline-flex gap-px">
      <Button
        variant="secondary"
        disabled={isSubmitting}
        onClick={() => decide("reject-once")}
        aria-keyshortcuts="3"
        kbd="3"
        data-testid="permission-reject-once"
      >
        {terminalRejectOnceLabel()}
      </Button>
      {offersRejectAlways ? (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                size="icon"
                variant="quiet"
                className="group/reject-menu"
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
              Never allow
              <DropdownMenuShortcut>4</DropdownMenuShortcut>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ) : null}
    </div>
  );
}
