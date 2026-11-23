import { Info, Link2, Plus, RefreshCw, Sparkles } from "@compozy/ui/icons";

import {
  Button,
  CodeBlock,
  DialogFooter,
  EntityDialogFooter,
  ListingRow,
  Pill,
  Spinner,
  StatusDot,
} from "@compozy/ui";

import type { AccountUnaddedComputerFixture } from "../mocks/remote-computers-fixtures";
import {
  LINK_WITH_COMMAND_FIXTURE,
  type AddComputerMethod,
  type AddComputerSshStage,
} from "../mocks/remote-computers-fixtures-lane-b";
import { ComputerPlatformIcon } from "./computer-platform-icon";

export interface AddComputerAccountPathProps {
  accountEmail: string;
  isFreePlan: boolean;
  unaddedComputers: readonly AccountUnaddedComputerFixture[];
  addedIds: readonly string[];
  onAdd: (computer: AccountUnaddedComputerFixture) => void;
  onUpgrade?: () => void;
}

/** "On your account" path inside `AddComputerDialog`: one-click add for linked computers. */
export function AddComputerAccountPath({
  accountEmail,
  isFreePlan,
  unaddedComputers,
  addedIds,
  onAdd,
  onUpgrade,
}: AddComputerAccountPathProps) {
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-small-body font-semibold text-fg-strong">
          Available on {accountEmail}
        </h3>
        <span className="text-meta text-muted-foreground">
          Linked elsewhere, not added on this device yet
        </span>
      </div>

      <div className="overflow-hidden rounded-lg border border-line bg-card">
        {(() => {
          const addedSet = new Set(addedIds);
          return unaddedComputers.map(item => {
            const isAdded = addedSet.has(item.id);
            const hostedBlocked = isFreePlan && item.hostedOnly;
            return (
              <ListingRow
                className="min-h-14 border-b border-line-soft py-3 last:border-b-0"
                interactive={false}
                key={item.id}
              >
                <ListingRow.Icon>
                  <ComputerPlatformIcon
                    iconKind={item.iconKind}
                    platform={item.platform}
                    size="sm"
                    tone="muted"
                  />
                </ListingRow.Icon>
                <ListingRow.Main>
                  <ListingRow.Name>
                    <ListingRow.Title>{item.name}</ListingRow.Title>
                  </ListingRow.Name>
                  <ListingRow.Meta className="mt-1">
                    <span>{isAdded ? "Added" : "Not added"}</span>
                    <ListingRow.MetaDot />
                    <span className="inline-flex items-center gap-1.5">
                      <StatusDot size="sm" tone={item.online ? "success" : "faint"} />
                      <span>{item.online ? "Online" : "Offline"}</span>
                    </span>
                    {hostedBlocked ? (
                      <>
                        <ListingRow.MetaDot />
                        <span className="text-warning">
                          Only reachable through the hosted route (Pro)
                        </span>
                      </>
                    ) : null}
                  </ListingRow.Meta>
                </ListingRow.Main>
                <ListingRow.Trail>
                  {isAdded ? (
                    <Pill size="xs" tone="success">
                      Added
                    </Pill>
                  ) : hostedBlocked ? (
                    <Button onClick={onUpgrade} size="sm" type="button" variant="secondary">
                      <Sparkles aria-hidden="true" />
                      Upgrade
                    </Button>
                  ) : (
                    <Button onClick={() => onAdd(item)} size="sm" type="button" variant="secondary">
                      <Plus aria-hidden="true" />
                      Add
                    </Button>
                  )}
                </ListingRow.Trail>
              </ListingRow>
            );
          });
        })()}
      </div>
    </div>
  );
}

export interface AddComputerCommandPathProps {
  accountEmail: string;
  detected: boolean;
  addedIds: readonly string[];
  onAdd: (computer: AccountUnaddedComputerFixture) => void;
}

/** "Link with a command" path inside `AddComputerDialog`: copyable command + live watcher. */
export function AddComputerCommandPath({
  accountEmail,
  detected,
  addedIds,
  onAdd,
}: AddComputerCommandPathProps) {
  const detectedComputer = LINK_WITH_COMMAND_FIXTURE.detectedComputer;
  const isAdded = addedIds.includes(detectedComputer.id);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <h3 className="text-small-body font-semibold text-fg-strong">
          Run this command on the other computer
        </h3>
        <p className="text-small-body text-muted-foreground">
          Works on macOS, Linux, and Windows. It signs in to{" "}
          <span className="font-medium text-fg">{accountEmail}</span> and turns on remote access
          automatically.
        </p>
        <CodeBlock
          code={LINK_WITH_COMMAND_FIXTURE.command}
          copyable
          density="compact"
          language="bash"
          showPrompt
        />
      </div>

      <div className="overflow-hidden rounded-lg border border-line bg-sunken/70">
        {detected ? (
          <ListingRow className="border-b-0 py-3" interactive={false}>
            <ListingRow.Icon>
              <ComputerPlatformIcon
                iconKind={detectedComputer.iconKind}
                platform={detectedComputer.platform}
                size="sm"
                tone="muted"
              />
            </ListingRow.Icon>
            <ListingRow.Main>
              <ListingRow.Name>
                <ListingRow.Title>{detectedComputer.name}</ListingRow.Title>
                <Pill size="xs" tone="success">
                  Just linked
                </Pill>
              </ListingRow.Name>
              <ListingRow.Meta className="mt-1">
                <span>{isAdded ? "Added" : "Not added"}</span>
                <ListingRow.MetaDot />
                <span className="inline-flex items-center gap-1.5">
                  <StatusDot size="sm" tone="success" />
                  <span>Online</span>
                </span>
              </ListingRow.Meta>
            </ListingRow.Main>
            <ListingRow.Trail>
              {isAdded ? (
                <Pill size="xs" tone="success">
                  Added
                </Pill>
              ) : (
                <Button onClick={() => onAdd(detectedComputer)} size="sm" type="button">
                  <Plus aria-hidden="true" />
                  Add
                </Button>
              )}
            </ListingRow.Trail>
          </ListingRow>
        ) : (
          <div className="flex items-center gap-2.5 px-4 py-3 text-small-body text-muted-foreground">
            <Spinner className="size-3.5 shrink-0" />
            <span>Listening for a new computer on {accountEmail}…</span>
          </div>
        )}
      </div>
    </div>
  );
}

function SingleActionFooter({
  hint,
  actionLabel,
  variant = "outline",
  onAction,
}: {
  hint?: string;
  actionLabel: string;
  variant?: "outline" | "default";
  onAction: () => void;
}) {
  return (
    <DialogFooter className="min-h-editor-footer items-center gap-3" variant="ruled">
      {hint ? (
        <div className="flex min-w-0 flex-1 items-center gap-2 overflow-hidden text-form-hint text-muted-foreground">
          <Info aria-hidden="true" className="size-3.5 shrink-0 text-faint" />
          <span className="block min-w-0 flex-1 truncate">{hint}</span>
        </div>
      ) : null}
      <Button onClick={onAction} type="button" variant={variant}>
        {actionLabel}
      </Button>
    </DialogFooter>
  );
}

export interface AddComputerFooterProps {
  method: AddComputerMethod;
  sshStage: AddComputerSshStage;
  accountEmail: string;
  hostLabel: string;
  pickerBlocked: boolean;
  commandDetected: boolean;
  linkedCommitReached: boolean;
  onClose: () => void;
  onRequestCancel: () => void;
  onStartSsh: () => void;
  onRetry: () => void;
  onUpgrade?: () => void;
  onOpenComputer?: (name: string) => void;
  onSwitchToCommand: () => void;
  onRemoveLinked: () => void;
}

/** Context-sensitive footer for `AddComputerDialog` reflecting setup phase & Rule 19. */
export function AddComputerFooter({
  method,
  sshStage,
  accountEmail,
  hostLabel,
  pickerBlocked,
  commandDetected,
  linkedCommitReached,
  onClose,
  onRequestCancel,
  onStartSsh,
  onRetry,
  onUpgrade,
  onOpenComputer,
  onSwitchToCommand,
  onRemoveLinked,
}: AddComputerFooterProps) {
  if (method === "account") {
    return (
      <SingleActionFooter
        actionLabel="Done"
        hint={`Linked to ${accountEmail}. Adding connects this app right away.`}
        onAction={onClose}
        variant="default"
      />
    );
  }

  if (method === "command") {
    return (
      <SingleActionFooter
        actionLabel={commandDetected ? "Done" : "Close"}
        hint="Keep this dialog open or add the computer later under On your account."
        onAction={onClose}
        variant={commandDetected ? "default" : "outline"}
      />
    );
  }

  if (sshStage === "picker") {
    return (
      <EntityDialogFooter
        cancelLabel="Cancel"
        hint="Checks your plan and release signature before touching the host."
        onCancel={onClose}
        onPrimary={onStartSsh}
        primaryDisabled={pickerBlocked}
        primaryIcon={Plus}
        primaryLabel="Add computer"
      />
    );
  }

  if (sshStage === "plan_limit") {
    return (
      <EntityDialogFooter
        cancelLabel="Cancel"
        hint="Unlinking your existing remote computer also frees your Free slot."
        onCancel={onClose}
        onPrimary={onUpgrade}
        primaryIcon={Sparkles}
        primaryLabel="Upgrade to Pro"
      />
    );
  }

  if (sshStage === "failed") {
    return (
      <EntityDialogFooter
        cancelLabel="Cancel"
        hint="Retry resumes from the failed step without repeating completed work."
        onCancel={onClose}
        onPrimary={onRetry}
        primaryIcon={RefreshCw}
        primaryLabel="Retry"
      />
    );
  }

  if (sshStage === "host_key_changed") {
    return (
      <EntityDialogFooter
        cancelLabel="Cancel"
        onCancel={onClose}
        onPrimary={onRetry}
        primaryIcon={RefreshCw}
        primaryLabel="Retry"
      />
    );
  }

  if (sshStage === "unsupported") {
    return (
      <EntityDialogFooter
        cancelLabel="Cancel"
        onCancel={onClose}
        onPrimary={onSwitchToCommand}
        primaryIcon={Link2}
        primaryLabel="Link with a command"
      />
    );
  }

  if (sshStage === "done") {
    return (
      <EntityDialogFooter
        cancelLabel="Done"
        hint={`${hostLabel} is ready in your Computers list and Run on picker.`}
        onCancel={onClose}
        onPrimary={() => onOpenComputer?.(hostLabel)}
        primaryLabel={`Open ${hostLabel}`}
      />
    );
  }

  if (linkedCommitReached) {
    return (
      <EntityDialogFooter
        cancelLabel="Close"
        hint="This computer is already linked. Remove it instead."
        onCancel={onClose}
        onPrimary={onRemoveLinked}
        primaryLabel="Remove"
      />
    );
  }

  if (sshStage === "trust_host" || sshStage === "password" || sshStage === "agent_secret") {
    return (
      <SingleActionFooter
        actionLabel="Cancel setup"
        hint="Cancelling stops setup and frees your computer slot."
        onAction={onRequestCancel}
        variant="outline"
      />
    );
  }

  return (
    <EntityDialogFooter
      cancelLabel="Cancel setup"
      hint="Cancelling stops setup and frees your computer slot."
      isSaving={sshStage === "running"}
      onCancel={onRequestCancel}
      primaryDisabled
      primaryLabel="Adding computer…"
    />
  );
}
