import { useState } from "react";
import { Pencil, Plus, Sparkles, Trash2 } from "@compozy/ui/icons";

import {
  Button,
  Checkbox,
  cn,
  ConfirmDialog,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Field,
  FieldLabel,
  Input,
  ListingRow,
  StatusDot,
} from "@compozy/ui";

import { SettingsGroup } from "@/systems/settings/components/settings-group";

import type {
  AccountStateFixture,
  AccountUnaddedComputerFixture,
  ComputerFixture,
  LocalComputerFixture,
} from "../mocks/remote-computers-types";
import { ComputerPlatformIcon } from "./computer-platform-icon";

export interface AccountComputersSectionProps {
  account: AccountStateFixture;
  computers: readonly AccountUnaddedComputerFixture[];
  addingComputerId?: string | null;
  onAddAccountComputer: (computer: AccountUnaddedComputerFixture) => void;
  onUpgrade?: () => void;
}

function AccountComputerMeta({
  hostedOnlyBlocked,
  online,
}: {
  hostedOnlyBlocked: boolean;
  online: boolean;
}) {
  return (
    <ListingRow.Meta className="mt-1">
      <span>Not added</span>
      <ListingRow.MetaDot />
      <span className="inline-flex items-center gap-1.5">
        <StatusDot size="sm" tone={online ? "success" : "faint"} />
        <span>{online ? "Online" : "Offline"}</span>
      </span>
      {hostedOnlyBlocked ? (
        <>
          <ListingRow.MetaDot />
          <span className="text-warning">
            This computer can only be reached through the hosted route (Pro)
          </span>
        </>
      ) : null}
    </ListingRow.Meta>
  );
}

/** Block 3 of Settings → Computers (`_uiux.md` U1): account-linked computers not yet added here. */
export function AccountComputersSection({
  account,
  computers,
  addingComputerId = null,
  onAddAccountComputer,
  onUpgrade,
}: AccountComputersSectionProps) {
  if (!account.signedIn || computers.length === 0) return null;

  return (
    <SettingsGroup
      data-testid="computers-on-account"
      description={`Linked to ${account.email} from another device or command.`}
      title="On your account"
    >
      {computers.map(computer => {
        const hostedOnlyBlocked = account.plan === "free" && computer.hostedOnly;
        const isAdding = addingComputerId === computer.id;
        return (
          <ListingRow
            className="min-h-14 py-3"
            data-account-computer-id={computer.id}
            interactive={false}
            key={computer.id}
          >
            <ListingRow.Icon>
              <ComputerPlatformIcon
                iconKind={computer.iconKind}
                platform={computer.platform}
                size="sm"
                tone="muted"
              />
            </ListingRow.Icon>
            <ListingRow.Main>
              <ListingRow.Name>
                <ListingRow.Title>{computer.name}</ListingRow.Title>
              </ListingRow.Name>
              <AccountComputerMeta hostedOnlyBlocked={hostedOnlyBlocked} online={computer.online} />
            </ListingRow.Main>
            <ListingRow.Trail>
              {hostedOnlyBlocked && onUpgrade ? (
                <Button onClick={onUpgrade} size="sm" type="button" variant="secondary">
                  <Sparkles aria-hidden="true" />
                  Upgrade
                </Button>
              ) : (
                <Button
                  disabled={isAdding || hostedOnlyBlocked}
                  onClick={() => onAddAccountComputer(computer)}
                  size="sm"
                  type="button"
                  variant="secondary"
                >
                  <Plus aria-hidden="true" />
                  {isAdding ? "Adding…" : "Add"}
                </Button>
              )}
            </ListingRow.Trail>
          </ListingRow>
        );
      })}
    </SettingsGroup>
  );
}

export interface RemoveComputerDialogProps {
  computer: ComputerFixture | null;
  isFreePlan: boolean;
  isPending?: boolean;
  onClose: () => void;
  onConfirm: (computer: ComputerFixture, options: { unlink: boolean; uninstall: boolean }) => void;
}

export function RemoveComputerDialog({
  computer,
  isFreePlan,
  isPending = false,
  onClose,
  onConfirm,
}: RemoveComputerDialogProps) {
  const [draft, setDraft] = useState<{ id: string; unlink: boolean; uninstall: boolean } | null>(
    null
  );

  if (!computer) return null;

  const unlink = draft && draft.id === computer.id ? draft.unlink : isFreePlan;
  const uninstall = draft && draft.id === computer.id ? draft.uninstall : false;

  return (
    <ConfirmDialog
      body={
        <div className="flex flex-col gap-3">
          <label
            className="flex cursor-pointer items-start gap-2.5"
            htmlFor="remove-computer-unlink"
          >
            <Checkbox
              checked={unlink}
              className="mt-0.5"
              id="remove-computer-unlink"
              onCheckedChange={next =>
                setDraft({ id: computer.id, unlink: Boolean(next), uninstall })
              }
            />
            <span className="flex flex-col gap-0.5">
              <span className="text-small-body font-medium text-fg">
                Also unlink from my account
              </span>
              <span className="text-meta text-muted-foreground">
                {isFreePlan
                  ? "Frees your free computer slot immediately."
                  : "Removes this computer from On your account and deletes its hosted route."}
              </span>
            </span>
          </label>

          {computer.originKind === "ssh" && computer.sshTarget ? (
            <label
              className="flex cursor-pointer items-start gap-2.5"
              htmlFor="remove-computer-uninstall"
            >
              <Checkbox
                checked={uninstall}
                className="mt-0.5"
                id="remove-computer-uninstall"
                onCheckedChange={next =>
                  setDraft({ id: computer.id, unlink, uninstall: Boolean(next) })
                }
              />
              <span className="flex flex-col gap-0.5">
                <span className="text-small-body font-medium text-fg">
                  Also uninstall CompozyOS from{" "}
                  <span className="font-mono text-mono-id">{computer.sshTarget}</span>
                </span>
                <span className="text-meta text-muted-foreground">
                  Stops the remote CompozyOS service and removes its binary from the host.
                </span>
              </span>
            </label>
          ) : null}
        </div>
      }
      cancelLabel="Cancel"
      confirmLabel="Remove"
      description={`Agents already running on ${computer.name} keep running. This app forgets its routes, credentials, and session list.`}
      icon={Trash2}
      isPending={isPending}
      onConfirm={() => {
        onConfirm(computer, { unlink, uninstall });
      }}
      onOpenChange={open => {
        if (!open) onClose();
      }}
      open
      title={`Remove ${computer.name}?`}
      tone="destructive"
    />
  );
}

export interface RenameComputerDialogProps {
  computer: ComputerFixture | null;
  isPending?: boolean;
  onClose: () => void;
  onConfirm: (computer: ComputerFixture, nextName: string) => void;
}

export function RenameComputerDialog({
  computer,
  isPending = false,
  onClose,
  onConfirm,
}: RenameComputerDialogProps) {
  const [draft, setDraft] = useState<{ id: string; name: string } | null>(null);

  if (!computer) return null;

  const name = draft && draft.id === computer.id ? draft.name : computer.name;
  const trimmed = name.trim();
  const canSave = trimmed.length > 0 && trimmed !== computer.name && !isPending;

  return (
    <Dialog
      onOpenChange={open => {
        if (!open) onClose();
      }}
      open
    >
      <DialogContent
        className={cn(dialogShellClass("sm"), "flex flex-col")}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description="Change the display name shown in your Computers list, Run on picker, and sidebar."
          eyebrow="Computers"
          icon={Pencil}
          onClose={onClose}
          title={`Rename ${computer.name}`}
        />
        <form
          onSubmit={event => {
            event.preventDefault();
            if (canSave) onConfirm(computer, trimmed);
          }}
        >
          <EntityDialogBody>
            <Field>
              <FieldLabel htmlFor="rename-computer-name">Display name</FieldLabel>
              <Input
                autoFocus
                id="rename-computer-name"
                onChange={event => setDraft({ id: computer.id, name: event.target.value })}
                placeholder={computer.name}
                value={name}
              />
            </Field>
          </EntityDialogBody>
          <EntityDialogFooter
            cancelLabel="Cancel"
            isSaving={isPending}
            onCancel={onClose}
            onPrimary={() => {
              if (canSave) onConfirm(computer, trimmed);
            }}
            primaryDisabled={!canSave}
            primaryLabel="Save"
          />
        </form>
      </DialogContent>
    </Dialog>
  );
}

export interface DisableLocalAgentsDialogProps {
  open: boolean;
  local: LocalComputerFixture;
  onClose: () => void;
  onConfirm: () => void;
}

export function DisableLocalAgentsDialog({
  open,
  local,
  onClose,
  onConfirm,
}: DisableLocalAgentsDialogProps) {
  if (!open) return null;
  const count = local.activeLocalSessions;
  return (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmLabel="Stop running agents"
      description={
        count > 0
          ? `${count} ${count === 1 ? "session is" : "sessions are"} running here. They keep running until they finish; new sessions go to your other computers.`
          : "Existing local sessions stay readable; new sessions go to your other computers."
      }
      onConfirm={onConfirm}
      onOpenChange={next => {
        if (!next) onClose();
      }}
      open
      title="Stop running agents on this computer?"
      tone="warning"
    />
  );
}
