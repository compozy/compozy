import { ExternalLink, ShieldAlert, Trash2, Unplug } from "@compozy/ui/icons";

import { Button, ConfirmDialog } from "@compozy/ui";

import type { AccountStateFixture } from "@/systems/computers/mocks/remote-computers-fixtures";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

export interface AccountDeleteSectionProps {
  account: AccountStateFixture;
  currentDeviceIsAdmin: boolean;
  onManageBilling: () => void;
  onRequestDelete: () => void;
}

/**
 * Bottom Delete account block (`_uiux.md` U3, US-028):
 * - Non-admin device: button absent, shows admin-device requirement note
 * - Active Pro subscription: button replaced by Cancel Pro notice + Manage billing
 * - Free admin device: consequence text + destructive Delete account trigger
 */
export function AccountDeleteSection({
  account,
  currentDeviceIsAdmin,
  onManageBilling,
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

  if (account.plan === "pro") {
    return (
      <SettingsGroup data-testid="account-delete" title="Delete account">
        <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-3.5">
          <p className="text-small-body text-muted-foreground">
            Cancel Pro before deleting your account.
          </p>
          <Button onClick={onManageBilling} size="sm" type="button" variant="secondary">
            <ExternalLink aria-hidden="true" />
            Manage billing
          </Button>
        </div>
      </SettingsGroup>
    );
  }

  return (
    <SettingsGroup data-testid="account-delete" title="Delete account">
      <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-3.5">
        <p className="text-small-body text-muted-foreground">
          Unlinks your computers and signs out every installation. Your local sessions and projects
          stay on this computer.
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
  onClose,
  onConfirm,
}: AccountSignOutDialogProps) {
  if (!open) return null;

  return (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmLabel="Sign out"
      description="CompozyOS needs you to sign in again before you can use it on this computer. Running agents finish their current work."
      icon={Unplug}
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
  onClose: () => void;
  onConfirm: () => void;
}

/**
 * Account deletion confirmation dialog (`_uiux.md` U3, US-028 AC-3):
 * lists what happens and requires typing the account email to confirm.
 */
export function AccountDeleteDialog({
  open,
  email,
  linkedComputersCount,
  onClose,
  onConfirm,
}: AccountDeleteDialogProps) {
  if (!open) return null;

  const unlinkLabel =
    linkedComputersCount === 1 ? "Unlinks 1 computer" : `Unlinks ${linkedComputersCount} computers`;

  return (
    <ConfirmDialog
      body={
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
      }
      cancelLabel="Cancel"
      confirmInputProps={{
        "aria-label": "Confirm your email",
        placeholder: email,
      }}
      confirmLabel="Delete account"
      confirmTyping={email}
      description="This can't be undone."
      icon={Trash2}
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
