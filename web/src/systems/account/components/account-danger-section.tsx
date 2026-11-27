import { ExternalLink, ShieldAlert, Trash2, TriangleAlert, Unplug } from "@compozy/ui/icons";

import { Alert, AlertDescription, AlertTitle, Button, ConfirmDialog } from "@compozy/ui";

import type { AccountStateFixture } from "@/systems/computers/mocks/remote-computers-fixtures";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

import {
  COMPOZY_REFUNDS_URL,
  describeDeleteAccountProImpact,
  type AccountDeleteProInfo,
} from "../lib/account-view";

export interface AccountDeleteProBlockProps {
  subscription: AccountDeleteProInfo;
  /** Optional clock override for deterministic refund-window rendering in tests. */
  now?: Date;
}

/**
 * Reusable Pro cancellation & 14-day refund summary block for the Delete account
 * dialog (`_uiux.md` U3, US-028 AC-4/EC-1, Rule 27; reused directly by S15).
 */
export function AccountDeleteProBlock({ subscription, now }: AccountDeleteProBlockProps) {
  const impact = describeDeleteAccountProImpact(subscription, now);

  return (
    <Alert className="gap-y-1.5 py-3" data-testid="account-delete-pro-block" variant="warning">
      <TriangleAlert aria-hidden="true" />
      <AlertTitle>{impact.heading}</AlertTitle>
      <AlertDescription className="flex flex-col gap-1.5">
        <p>{impact.periodLine}</p>
        {impact.refundLine ? (
          <p className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <span>{impact.refundLine}</span>
            {impact.showRefundPolicyLink ? (
              <a
                className="inline-flex items-center gap-1 font-medium text-fg underline underline-offset-3 hover:text-fg-strong"
                href={COMPOZY_REFUNDS_URL}
                rel="noreferrer"
                target="_blank"
              >
                Refund policy
                <ExternalLink aria-hidden="true" className="size-3" />
              </a>
            ) : null}
          </p>
        ) : null}
      </AlertDescription>
    </Alert>
  );
}

export interface AccountDeleteSectionProps {
  account: AccountStateFixture;
  currentDeviceIsAdmin: boolean;
  onManageBilling?: () => void;
  onRequestDelete: () => void;
}

/**
 * Bottom Delete account block (`_uiux.md` U3, US-028, Rule 27):
 * - Non-admin device: button absent, shows admin-device requirement note
 * - Admin device (Free or Pro): consequence text + destructive Delete account trigger;
 *   active Pro never blocks deletion and is cancelled in the dialog.
 */
export function AccountDeleteSection({
  account,
  currentDeviceIsAdmin,
  onRequestDelete,
}: AccountDeleteSectionProps) {
  if (!currentDeviceIsAdmin) {
    return (
      <SettingsGroup data-testid="account-delete" title="Delete account">
        <div className="flex items-center gap-2.5 px-4 py-3.5 text-small-body text-muted-foreground">
          <ShieldAlert aria-hidden="true" className="size-4 shrink-0 text-subtle" />
          <span>Delete your account from a device trusted for admin.</span>
        </div>
      </SettingsGroup>
    );
  }

  return (
    <SettingsGroup data-testid="account-delete" title="Delete account">
      <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-3.5">
        <p className="text-small-body text-muted-foreground">
          {account.plan === "pro"
            ? "Cancels Pro, unlinks your computers, and signs out every installation. Your local sessions and projects stay on this computer."
            : "Unlinks your computers and signs out every installation. Your local sessions and projects stay on this computer."}
        </p>
        <Button onClick={onRequestDelete} size="sm" type="button" variant="destructive">
          Delete account
        </Button>
      </div>
    </SettingsGroup>
  );
}

export interface AccountSignOutDialogProps {
  open: boolean;
  installationLinked: boolean;
  isPending?: boolean;
  error?: string | null;
  onClose: () => void;
  onConfirm: () => void;
}

/**
 * Sign-out confirmation dialog (`_uiux.md` U3, US-001 AC-4/AC-5):
 * adds the unlink warning when this computer is also linked for remote access.
 */
export function AccountSignOutDialog({
  open,
  installationLinked,
  isPending = false,
  error = null,
  onClose,
  onConfirm,
}: AccountSignOutDialogProps) {
  if (!open) return null;

  return (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmLabel="Sign out"
      description="CompozyOS needs you to sign in again before you can use it on this computer. Running agents finish their current work."
      error={error}
      icon={Unplug}
      isPending={isPending}
      note={installationLinked ? "This also unlinks this computer from remote access." : undefined}
      noteTone="warning"
      onConfirm={onConfirm}
      onOpenChange={next => {
        if (!next) onClose();
      }}
      open={open}
      title="Sign out of CompozyOS?"
      tone="warning"
    />
  );
}

export interface AccountDeleteDialogProps {
  open: boolean;
  email: string;
  linkedComputersCount: number;
  /** When present, renders `AccountDeleteProBlock` and updates the confirm button copy. */
  subscription?: AccountDeleteProInfo | null;
  now?: Date;
  isPending?: boolean;
  error?: string | null;
  onClose: () => void;
  onConfirm: () => void;
}

/**
 * Account deletion confirmation dialog (`_uiux.md` U3, US-028 AC-3/AC-4):
 * lists what happens, adds the Pro cancellation/refund block when Pro is active,
 * and requires typing the account email to confirm.
 */
export function AccountDeleteDialog({
  open,
  email,
  linkedComputersCount,
  subscription = null,
  now,
  isPending = false,
  error = null,
  onClose,
  onConfirm,
}: AccountDeleteDialogProps) {
  if (!open) return null;

  const unlinkLabel =
    linkedComputersCount === 1 ? "Unlinks 1 computer" : `Unlinks ${linkedComputersCount} computers`;
  const confirmLabel = subscription ? "Delete account and cancel Pro" : "Delete account";

  return (
    <ConfirmDialog
      body={
        <div className="flex flex-col gap-3">
          {subscription ? <AccountDeleteProBlock now={now} subscription={subscription} /> : null}
          <ul className="flex flex-col gap-1.5 text-small-body text-muted-foreground">
            <li className="flex items-start gap-2">
              <span aria-hidden="true" className="mt-2 size-1 shrink-0 rounded-full bg-faint" />
              <span>{unlinkLabel}</span>
            </li>
            <li className="flex items-start gap-2">
              <span aria-hidden="true" className="mt-2 size-1 shrink-0 rounded-full bg-faint" />
              <span>Signs out every installation</span>
            </li>
            <li className="flex items-start gap-2">
              <span aria-hidden="true" className="mt-2 size-1 shrink-0 rounded-full bg-faint" />
              <span>Your local sessions and projects stay on this computer</span>
            </li>
          </ul>
        </div>
      }
      cancelLabel="Cancel"
      confirmInputProps={{
        "aria-label": "Confirm your email",
        placeholder: email,
      }}
      confirmLabel={confirmLabel}
      confirmTyping={email}
      description="This can't be undone."
      error={error}
      icon={Trash2}
      isPending={isPending}
      onConfirm={onConfirm}
      onOpenChange={next => {
        if (!next) onClose();
      }}
      open={open}
      title="Delete your CompozyOS account?"
      tone="destructive"
    />
  );
}
