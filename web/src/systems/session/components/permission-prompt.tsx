import { Check, ShieldAlert, ShieldCheck, ShieldOff, X } from "lucide-react";

import { Button, CodeBlock, Eyebrow, cn } from "@agh/ui";

import type { PermissionDecision } from "../adapters/session-api";
import { useSessionPermissionDecision } from "../hooks/use-session-permission-decision";
import { toPermissionRequest } from "../lib/message-parts";
import type { AghPermissionData, PermissionRequest } from "../types";

export interface PermissionPromptProps {
  permission: PermissionRequest;
  sessionId: string;
  workspaceId: string;
  onResolved?: () => void;
}

type PromptTone = "warning" | "danger";

/**
 * High-stakes tools that affect the filesystem or the network.
 * Permission prompts for these tools render with the `danger` tone (icon signal)
 * so the operator cannot scroll past them without a deliberate decision.
 */
const HIGH_STAKES_TOOLS = new Set([
  "Bash",
  "Write",
  "Edit",
  "NotebookEdit",
  "WebFetch",
  "WebSearch",
]);

const FALLBACK_PERMISSION_DECISIONS = [
  "allow-once",
  "allow-always",
  "reject-once",
  "reject-always",
] satisfies PermissionDecision[];

function promptToneFor(toolName: string | undefined): PromptTone {
  if (toolName && HIGH_STAKES_TOOLS.has(toolName)) return "danger";
  return "warning";
}

function normalizePermissionDecision(value: string | undefined): PermissionDecision | null {
  switch (value?.trim()) {
    case "allow-once":
      return "allow-once";
    case "allow-always":
      return "allow-always";
    case "reject-once":
      return "reject-once";
    case "reject-always":
      return "reject-always";
    default:
      return null;
  }
}

function permissionDecisionOptions(permission: PermissionRequest): PermissionDecision[] {
  if (permission.supportedDecisions == null || permission.supportedDecisions.length === 0) {
    return FALLBACK_PERMISSION_DECISIONS;
  }
  const supported = new Set(permission.supportedDecisions);
  const filtered = FALLBACK_PERMISSION_DECISIONS.filter(decision => supported.has(decision));
  return filtered.length > 0 ? filtered : FALLBACK_PERMISSION_DECISIONS;
}

export function PermissionPrompt({
  permission,
  sessionId,
  workspaceId,
  onResolved,
}: PermissionPromptProps) {
  const { decide, isResolved, isSubmitting } = useSessionPermissionDecision({
    workspaceId,
    sessionId,
    permission,
    onResolved,
  });

  const tone = promptToneFor(permission.toolName);
  const isHighStakes = tone === "danger";
  const decisionOptions = permissionDecisionOptions(permission);

  return isResolved ? null : (
    <div
      className="sticky top-2 z-10 w-full py-1.5"
      data-testid="permission-prompt"
      data-tone={tone}
      data-sticky="true"
      role="region"
      aria-label="Permission required"
    >
      <div
        className="flex w-full min-w-0 gap-2.5 rounded-md border border-line bg-canvas-soft px-3 py-2.5"
        data-testid="permission-prompt-card"
      >
        <PermissionToneTile tone={tone} />
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <header className="flex flex-col gap-0.5">
            <Eyebrow
              data-testid="permission-prompt-eyebrow"
              className={cn(isHighStakes ? "text-danger" : "text-warning")}
            >
              Permission Required
            </Eyebrow>
            <p className="text-form-input leading-snug text-muted">
              The agent is requesting permission before it continues this turn.
            </p>
          </header>
          <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-form-label">
            <dt className="eyebrow text-muted">Tool</dt>
            <dd className="font-mono text-fg">{permission.toolName}</dd>
            {permission.action ? (
              <>
                <dt className="eyebrow text-muted">Action</dt>
                <dd className="text-fg">{permission.action}</dd>
              </>
            ) : null}
            {permission.resource ? (
              <>
                <dt className="eyebrow text-muted">Resource</dt>
                <dd className="min-w-0 truncate font-mono text-fg">{permission.resource}</dd>
              </>
            ) : null}
          </dl>
          {Object.keys(permission.toolInput).length > 0 ? (
            <CodeBlock
              code={JSON.stringify(permission.toolInput, null, 2)}
              copyable={false}
              data-testid="permission-tool-input"
              density="compact"
              showPrompt={false}
              truncateLines={6}
              wrapLines
            />
          ) : null}
          <div className="flex flex-wrap items-center gap-1.5 pt-0.5">
            {decisionOptions.includes("allow-once") ? (
              <Button
                variant="outline"
                size="sm"
                disabled={isSubmitting}
                onClick={() => decide("allow-once")}
                data-testid="permission-allow-once"
              >
                <Check className="size-3" />
                Allow once
              </Button>
            ) : null}
            {decisionOptions.includes("allow-always") ? (
              <Button
                variant="outline"
                size="sm"
                disabled={isSubmitting}
                onClick={() => decide("allow-always")}
                data-testid="permission-allow-always"
              >
                <ShieldCheck className="size-3" />
                Allow always
              </Button>
            ) : null}
            {decisionOptions.includes("reject-once") ? (
              <Button
                variant={isHighStakes ? "destructive" : "outline"}
                size="sm"
                disabled={isSubmitting}
                onClick={() => decide("reject-once")}
                data-testid="permission-reject-once"
              >
                <X className="size-3" />
                Reject once
              </Button>
            ) : null}
            {decisionOptions.includes("reject-always") ? (
              <Button
                variant="destructive"
                size="sm"
                disabled={isSubmitting}
                onClick={() => decide("reject-always")}
                data-testid="permission-reject-always"
              >
                <ShieldOff className="size-3" />
                Reject always
              </Button>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}

interface PermissionToneTileProps {
  tone: PromptTone;
}

function PermissionToneTile({ tone }: PermissionToneTileProps) {
  const isDanger = tone === "danger";
  return (
    <span
      data-testid="permission-prompt-tile"
      data-tone={tone}
      aria-hidden="true"
      className={cn(
        "flex size-6 shrink-0 items-center justify-center rounded-sm",
        "bg-elevated",
        isDanger ? "text-danger" : "text-warning"
      )}
    >
      <ShieldAlert className="size-3.5" strokeWidth={1.75} />
    </span>
  );
}

export function PermissionDataPart({
  data,
  sessionId,
  workspaceId,
}: {
  data: AghPermissionData;
  sessionId: string;
  workspaceId: string;
}) {
  const decision = normalizePermissionDecision(data.decision);
  const permission = toPermissionRequest(data);
  switch (decision) {
    case "allow-once":
    case "allow-always":
      return null;
    case "reject-once":
    case "reject-always":
      return <PermissionRejectedNotice permission={permission} />;
    default:
      return (
        <PermissionPrompt permission={permission} sessionId={sessionId} workspaceId={workspaceId} />
      );
  }
}

function PermissionRejectedNotice({ permission }: { permission: PermissionRequest }) {
  return (
    <div className="w-full py-1.5" data-testid="permission-rejected-notice" role="status">
      <div className="flex w-full min-w-0 items-start gap-2 rounded-md border border-line bg-canvas-soft px-3 py-2">
        <ShieldOff aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-danger" />
        <div className="min-w-0 flex-1">
          <div className="text-small-body font-medium text-fg-strong">Permission Rejected</div>
          <div className="mt-0.5 flex min-w-0 flex-wrap gap-x-2 gap-y-0.5 text-form-label text-muted">
            <span className="font-mono text-fg">{permission.toolName}</span>
            {permission.resource ? <span className="truncate">{permission.resource}</span> : null}
          </div>
        </div>
      </div>
    </div>
  );
}
