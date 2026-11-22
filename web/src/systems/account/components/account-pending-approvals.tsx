import { ArrowRight, Check, Clock, ShieldCheck, X } from "@compozy/ui/icons";

import { Button, Disclosure, ListingRow, Pill, Spinner, StatusDot } from "@compozy/ui";

import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import { KeyFingerprint } from "@/systems/computers/components/key-fingerprint";
import type {
  ApprovalState,
  PendingApprovalFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

export interface AccountPendingApprovalsSectionProps {
  approvals: readonly PendingApprovalFixture[];
  onApprove: (approvalId: string) => void;
  onDeny: (approvalId: string) => void;
}

const ACTIVE_RANK: Record<Extract<ApprovalState, "pending" | "delivering">, number> = {
  pending: 0,
  delivering: 1,
};

function DecidedStatusPill({ state }: { state: PendingApprovalFixture["state"] }) {
  if (state === "approved") {
    return (
      <Pill size="xs" tone="success">
        Approved
      </Pill>
    );
  }
  if (state === "denied") {
    return (
      <Pill size="xs" tone="destructive">
        Denied
      </Pill>
    );
  }
  return (
    <Pill size="xs" tone="neutral">
      Expired
    </Pill>
  );
}

/**
 * Pending and recent device approvals (`_uiux.md` U3, US-021 AC-3):
 * - Floats above Account → Devices so time-sensitive requests stay visible
 * - Matches Screen 02 treatment: inline fingerprint on the secondary line, no nested boxes
 * - Amber indicator only on actionable pending rows; decided/expired items collapse into a disclosure
 */
export function AccountPendingApprovalsSection({
  approvals,
  onApprove,
  onDeny,
}: AccountPendingApprovalsSectionProps) {
  if (approvals.length === 0) return null;

  const active = approvals
    .filter(
      (item): item is PendingApprovalFixture & { state: "pending" | "delivering" } =>
        item.state === "pending" || item.state === "delivering"
    )
    .sort((a, b) => {
      const rankDiff = ACTIVE_RANK[a.state] - ACTIVE_RANK[b.state];
      if (rankDiff !== 0) return rankDiff;
      if (a.state === "pending" && b.state === "pending" && a.canDecide !== b.canDecide) {
        return a.canDecide ? -1 : 1;
      }
      return 0;
    });

  const recent = approvals.filter(
    item => item.state === "approved" || item.state === "denied" || item.state === "expired"
  );

  const pendingCount = active.filter(item => item.state === "pending").length;

  return (
    <SettingsGroup
      action={
        pendingCount > 0 ? (
          <Pill size="xs" tone="warning">
            {pendingCount} waiting
          </Pill>
        ) : undefined
      }
      data-testid="account-pending-approvals"
      description={
        pendingCount > 0
          ? "Compare on the requesting device before approving."
          : "Decisions from the last 24 hours."
      }
      help="When a computer requires approval for new devices, its first sign-in holds for 10 minutes until approved on that computer or from a device it already trusts."
      title="Pending approvals"
    >
      {active.map(approval => {
        const isPending = approval.state === "pending";
        const isActionable = isPending && approval.canDecide;

        return (
          <ListingRow
            className="min-h-14 py-3"
            data-approval-id={approval.id}
            data-approval-state={approval.state}
            interactive={false}
            key={approval.id}
          >
            <ListingRow.Icon>
              <ComputerPlatformIcon
                deviceKind={approval.deviceKind}
                size="sm"
                tone={isActionable ? "default" : "muted"}
              />
            </ListingRow.Icon>

            <ListingRow.Main className="gap-1">
              <ListingRow.Name className="flex-wrap gap-y-1">
                {isActionable ? <StatusDot size="sm" tone="warning" /> : null}
                <ListingRow.Title>{approval.deviceName}</ListingRow.Title>
                <span className="inline-flex items-center gap-1 text-meta text-muted-foreground">
                  <ArrowRight aria-hidden="true" className="size-3 text-faint" />
                  <span className="font-medium text-fg">{approval.computerName}</span>
                </span>
                {approval.state === "delivering" ? (
                  <Pill form="plain" size="xs" tone="neutral">
                    <Spinner aria-hidden="true" className="size-3" />
                    Delivering to {approval.computerName}…
                  </Pill>
                ) : null}
              </ListingRow.Name>

              <ListingRow.Meta>
                <KeyFingerprint layout="inline" value={approval.keyFingerprint} />
                <ListingRow.MetaDot />
                <span className="inline-flex items-center gap-1">
                  <Clock aria-hidden="true" className="size-3 text-faint" />
                  <span>{approval.createdLabel}</span>
                </span>
              </ListingRow.Meta>

              {isPending && !approval.canDecide ? (
                <p className="text-meta text-muted-foreground">
                  Approve on {approval.computerName}, or from a device it already trusts.
                </p>
              ) : null}

              {approval.state === "delivering" ? (
                <p className="text-meta text-muted-foreground">
                  Signed decision sent. Waiting for {approval.computerName} to confirm.
                </p>
              ) : null}
            </ListingRow.Main>

            <ListingRow.Trail>
              {isPending ? (
                <div className="flex shrink-0 items-center gap-1.5">
                  <Button
                    disabled={!approval.canDecide}
                    onClick={() => onDeny(approval.id)}
                    size="sm"
                    title={
                      approval.canDecide
                        ? `Deny ${approval.deviceName}`
                        : `Approve on ${approval.computerName}, or from a device it already trusts.`
                    }
                    type="button"
                    variant="outline"
                  >
                    <X aria-hidden="true" />
                    Deny
                  </Button>
                  <Button
                    disabled={!approval.canDecide}
                    onClick={() => onApprove(approval.id)}
                    size="sm"
                    title={
                      approval.canDecide
                        ? `Approve ${approval.deviceName}`
                        : `Approve on ${approval.computerName}, or from a device it already trusts.`
                    }
                    type="button"
                    variant="default"
                  >
                    <Check aria-hidden="true" />
                    Approve
                  </Button>
                </div>
              ) : null}
            </ListingRow.Trail>
          </ListingRow>
        );
      })}

      {recent.length > 0 ? (
        <div className="border-t border-line-soft px-4 py-2.5 first:border-t-0">
          <Disclosure
            defaultOpen={false}
            label={
              <span className="text-small-body font-medium text-muted-foreground">
                Recent decisions ({recent.length})
              </span>
            }
            size="sm"
            variant="inline"
          >
            <div className="mt-1 flex flex-col divide-y divide-line-soft">
              {recent.map(item => (
                <div
                  className="flex flex-wrap items-center justify-between gap-3 py-2.5 first:pt-1 last:pb-0"
                  data-approval-id={item.id}
                  data-approval-state={item.state}
                  key={item.id}
                >
                  <div className="flex min-w-0 items-center gap-2.5">
                    <ComputerPlatformIcon deviceKind={item.deviceKind} size="xs" tone="muted" />
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-1.5">
                        <span className="truncate text-small-body font-medium text-fg">
                          {item.deviceName}
                        </span>
                        <ArrowRight aria-hidden="true" className="size-3 text-faint" />
                        <span className="text-small-body text-muted-foreground">
                          {item.computerName}
                        </span>
                        <DecidedStatusPill state={item.state} />
                      </div>
                      <div className="mt-0.5 flex flex-wrap items-center gap-1.5 text-eyebrow text-faint">
                        <KeyFingerprint
                          copyable={false}
                          layout="inline"
                          value={item.keyFingerprint}
                        />
                        <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
                        <span>{item.createdLabel}</span>
                        {item.state === "approved" ? (
                          <>
                            <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
                            <span className="inline-flex items-center gap-1">
                              <ShieldCheck aria-hidden="true" className="size-3 text-success" />
                              <span>Applied on {item.computerName}</span>
                            </span>
                          </>
                        ) : null}
                        {item.state === "expired" ? (
                          <>
                            <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
                            <span>No decision within 10 minutes</span>
                          </>
                        ) : null}
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </Disclosure>
        </div>
      ) : null}
    </SettingsGroup>
  );
}
