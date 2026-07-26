import {
  AlertTriangle,
  CheckCircle2,
  CircleHelp,
  ExternalLink,
  MinusCircle,
  RotateCw,
} from "lucide-react";

import {
  Button,
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemMedia,
  ItemTitle,
  OperationalLinksRow,
  Pill,
  Section,
  Spinner,
  type PillTone,
} from "@agh/ui";

import type {
  BridgeSetupChecklistItem,
  BridgeSetupProjection,
  BridgeSetupState,
} from "../lib/bridge-setup";
import type { BridgeVerifyCheck } from "../types";

interface BridgeSetupChecklistProps {
  isLifecyclePending: boolean;
  isRegistering: boolean;
  isVerifying: boolean;
  onEnableBridge?: () => void;
  onOpenEdit?: () => void;
  onRegisterWebhook: () => void;
  onRestartBridge?: () => void;
  onVerify: () => void;
  projection: BridgeSetupProjection | null;
}

const LABELS: Record<BridgeSetupChecklistItem["id"], string> = {
  provider: "Provider installed",
  secrets: "Required secrets bound",
  webhook: "Webhook configured",
  verified: "Live verification",
  enabled: "Bridge enabled",
  healthy: "Runtime healthy",
};

const DESCRIPTIONS: Record<BridgeSetupChecklistItem["id"], string> = {
  provider: "The configured provider matches this bridge.",
  secrets: "Every required provider secret has a vault binding.",
  webhook: "The provider can receive events at its public endpoint.",
  verified: "Provider identity, scopes, and webhook checks reflect the latest verification.",
  enabled: "The bridge is allowed to run and deliver messages.",
  healthy: "The runtime reports this bridge ready for work.",
};

function statePresentation(state: BridgeSetupState): {
  Icon: typeof CheckCircle2;
  iconClassName: string;
  label: string;
  tone: PillTone;
} {
  switch (state) {
    case "complete":
      return {
        Icon: CheckCircle2,
        iconClassName: "text-success",
        label: "COMPLETE",
        tone: "success",
      };
    case "action-needed":
      return {
        Icon: AlertTriangle,
        iconClassName: "text-warning",
        label: "ACTION NEEDED",
        tone: "warning",
      };
    case "unknown":
      return { Icon: CircleHelp, iconClassName: "text-muted", label: "UNKNOWN", tone: "neutral" };
    case "warn":
      return {
        Icon: AlertTriangle,
        iconClassName: "text-warning",
        label: "WARNING",
        tone: "warning",
      };
    case "fail":
      return { Icon: AlertTriangle, iconClassName: "text-danger", label: "FAILED", tone: "danger" };
    case "skipped":
      return { Icon: MinusCircle, iconClassName: "text-subtle", label: "SKIPPED", tone: "neutral" };
  }
}

function isResolvedState(state: BridgeSetupState): boolean {
  return state === "complete" || state === "skipped";
}

function checklistLabel(item: BridgeSetupChecklistItem, projection: BridgeSetupProjection): string {
  if (item.id === "webhook" && projection.profile?.webhookRegistration === "telegram") {
    return "Webhook registered";
  }
  return LABELS[item.id];
}

function checklistDescription(
  item: BridgeSetupChecklistItem,
  projection: BridgeSetupProjection
): string {
  const base = `${DESCRIPTIONS[item.id]}${item.remediation ? ` ${item.remediation}` : ""}`;
  if (
    item.id === "webhook" &&
    projection.profile?.webhookRegistration === "telegram" &&
    projection.facts.enabled
  ) {
    return `${base} Disable the bridge before registering its Telegram webhook.`;
  }
  return base;
}

function focusSecretSlots() {
  const secretSlots = document.getElementById("bridge-secret-slots");
  secretSlots?.focus();
  secretSlots?.scrollIntoView?.({ block: "start", behavior: "smooth" });
}

function SetupActions({
  item,
  isLifecyclePending,
  isRegistering,
  isVerifying,
  onEnableBridge,
  onOpenEdit,
  onRegisterWebhook,
  onRestartBridge,
  onVerify,
  projection,
}: Omit<BridgeSetupChecklistProps, "projection"> & {
  item: BridgeSetupChecklistItem;
  projection: BridgeSetupProjection;
}) {
  if (item.id === "secrets" && item.state !== "complete") {
    return (
      <Button onClick={focusSecretSlots} size="sm" type="button" variant="outline">
        Review secrets
      </Button>
    );
  }

  if (item.id === "webhook") {
    return (
      <>
        {item.state !== "complete" ? (
          <Button
            disabled={isLifecyclePending}
            onClick={onOpenEdit}
            size="sm"
            type="button"
            variant="outline"
          >
            Edit config
          </Button>
        ) : null}
        {projection.profile?.webhookRegistration === "telegram" ? (
          <Button
            data-testid="register-bridge-webhook-btn"
            disabled={
              isLifecyclePending ||
              isRegistering ||
              isVerifying ||
              projection.facts.enabled ||
              projection.facts.webhookPublicUrl === null ||
              !projection.checklist.some(
                candidate => candidate.id === "secrets" && isResolvedState(candidate.state)
              )
            }
            onClick={onRegisterWebhook}
            size="sm"
            type="button"
            variant="outline"
          >
            {isRegistering ? <Spinner aria-hidden="true" className="size-3" /> : null}
            {isRegistering
              ? "Registering…"
              : projection.facts.enabled
                ? "Disable first"
                : "Register webhook"}
          </Button>
        ) : null}
      </>
    );
  }

  if (item.id === "verified") {
    return (
      <Button
        data-testid="verify-bridge-btn"
        disabled={isLifecyclePending || isVerifying || isRegistering}
        onClick={onVerify}
        size="sm"
        type="button"
        variant="outline"
      >
        {isVerifying ? <Spinner aria-hidden="true" className="size-3" /> : null}
        {isVerifying ? "Verifying…" : item.state === "complete" ? "Verify again" : "Verify"}
      </Button>
    );
  }

  if (item.id === "enabled" && item.state !== "complete") {
    return (
      <Button
        data-testid="setup-enable-bridge-btn"
        disabled={isLifecyclePending}
        onClick={onEnableBridge}
        size="sm"
        type="button"
      >
        Enable
      </Button>
    );
  }

  if (item.id === "healthy" && item.state !== "complete" && item.state !== "skipped") {
    return (
      <Button
        disabled={isLifecyclePending}
        onClick={onRestartBridge}
        size="sm"
        type="button"
        variant="outline"
      >
        <RotateCw className="size-3" />
        Restart
      </Button>
    );
  }

  return null;
}

function GeneralCheck({ check }: { check: BridgeVerifyCheck }) {
  const state = check.status === "pass" ? "complete" : check.status;
  const presentation = statePresentation(state);
  return (
    <Item data-testid={`bridge-global-check-${check.check}`} size="sm" variant="outline">
      <ItemMedia variant="icon">
        <presentation.Icon aria-hidden="true" className={`size-4 ${presentation.iconClassName}`} />
      </ItemMedia>
      <ItemContent>
        <ItemTitle className="font-mono text-small-body">{check.check}</ItemTitle>
        {check.remediation ? (
          <ItemDescription className="line-clamp-none">{check.remediation}</ItemDescription>
        ) : null}
      </ItemContent>
      <ItemActions>
        <Pill mono tone={presentation.tone}>
          {presentation.label}
        </Pill>
      </ItemActions>
    </Item>
  );
}

function profileIssueMessage(issue: BridgeSetupProjection["profileIssues"][number]): string {
  if (issue.kind === "provider-mismatch") {
    return `Expected ${issue.expectedPlatform}; received ${issue.actualPlatform}.`;
  }
  return `Verification check ${issue.check} references unavailable secret slot ${issue.slot}.`;
}

function profileIssueKey(issue: BridgeSetupProjection["profileIssues"][number]): string {
  switch (issue.kind) {
    case "provider-mismatch":
      return `${issue.kind}-${issue.expectedPlatform}-${issue.actualPlatform}`;
    case "missing-secret-slot":
      return `${issue.kind}-${issue.check}-${issue.slot}`;
  }
}

export function BridgeSetupChecklist(props: BridgeSetupChecklistProps) {
  const { projection } = props;
  if (projection === null) {
    return (
      <Section label="Setup">
        <p className="rounded-md border border-line bg-canvas-soft px-4 py-3 text-small-body text-muted">
          Setup status is not available yet.
        </p>
      </Section>
    );
  }

  const resolvedCount = projection.checklist.filter(item => isResolvedState(item.state)).length;
  const firstUnresolved = projection.checklist.find(
    item => item.state !== "complete" && item.state !== "skipped"
  );

  return (
    <Section
      data-testid="bridge-setup-checklist"
      label="Setup"
      right={
        <Pill mono tone={resolvedCount === projection.checklist.length ? "success" : "neutral"}>
          {resolvedCount}/{projection.checklist.length}
        </Pill>
      }
    >
      <ItemGroup className="gap-2">
        {projection.checklist.map(item => {
          const presentation = statePresentation(item.state);
          const isNext = firstUnresolved?.id === item.id;
          return (
            <Item
              data-testid={`bridge-setup-item-${item.id}`}
              key={item.id}
              size="sm"
              variant={isNext ? "muted" : "outline"}
            >
              <ItemMedia variant="icon">
                <presentation.Icon
                  aria-hidden="true"
                  className={`size-4 ${presentation.iconClassName}`}
                />
              </ItemMedia>
              <ItemContent>
                <ItemTitle>
                  {checklistLabel(item, projection)}
                  {isNext ? (
                    <Pill mono tone="info">
                      NEXT
                    </Pill>
                  ) : null}
                </ItemTitle>
                <ItemDescription className="line-clamp-none">
                  {checklistDescription(item, projection)}
                </ItemDescription>
              </ItemContent>
              <ItemActions className="flex-wrap justify-end">
                <Pill mono tone={presentation.tone}>
                  {presentation.label}
                </Pill>
                <SetupActions {...props} item={item} projection={projection} />
              </ItemActions>
            </Item>
          );
        })}
      </ItemGroup>

      {projection.profile?.dashboardUrl ? (
        <OperationalLinksRow
          ariaLabel="Provider resources"
          className="mt-3"
          items={[
            {
              href: projection.profile.dashboardUrl,
              icon: ExternalLink,
              label: `${projection.profile.platform} dashboard`,
              target: "_blank",
            },
          ]}
        />
      ) : null}

      {projection.globalChecks.length > 0 ? (
        <div className="mt-4" data-testid="bridge-global-checks">
          <p className="mb-2 text-eyebrow uppercase tracking-mono text-muted">
            General verification checks
          </p>
          <ItemGroup className="gap-2">
            {projection.globalChecks.map(check => (
              <GeneralCheck check={check} key={check.check} />
            ))}
          </ItemGroup>
        </div>
      ) : null}

      {projection.profileIssues.length > 0 ? (
        <div
          className="mt-3 rounded-md border border-warning/40 bg-warning-tint px-4 py-3"
          data-testid="bridge-setup-profile-issues"
        >
          <p className="text-small-body font-medium text-warning">Setup mapping needs attention</p>
          <ul className="mt-2 list-disc space-y-1 pl-4 text-xs text-muted">
            {projection.profileIssues.map(issue => (
              <li key={profileIssueKey(issue)}>{profileIssueMessage(issue)}</li>
            ))}
          </ul>
        </div>
      ) : null}
    </Section>
  );
}
