import { useState } from "react";
import { TextCursorInput } from "lucide-react";

import {
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Field,
  FieldError,
  FieldLabel,
  Input,
  Spinner,
} from "@compozy/ui";

import { PROFILE_PERMANENT_LINE } from "../lib/profile-copy";
import { PERMANENT_PROFILE } from "../lib/profile-rows";
import type { RenameProfilePlan } from "../types";
import { ProfileRenamePlan } from "./profile-rename-plan";

export interface ProfileRenameDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  profile: string;
  newName: string;
  onNewNameChange: (next: string) => void;
  plan: RenameProfilePlan | undefined;
  planLoading: boolean;
  acceptedRepos: readonly string[];
  onToggleRepo: (workspaceId: string) => void;
  isPending: boolean;
  error?: string | null;
  onRename: (planRevision: string) => void;
}

/**
 * Rename a profile.
 *
 * `default` is permanent, so the field is absent rather than present-and-refusing
 * — a control that can never succeed is worse than no control.
 */
export function ProfileRenameDialog({
  open,
  onOpenChange,
  profile,
  newName,
  onNewNameChange,
  plan,
  planLoading,
  acceptedRepos,
  onToggleRepo,
  isPending,
  error = null,
  onRename,
}: ProfileRenameDialogProps) {
  const [touched, setTouched] = useState(false);
  const permanent = profile === PERMANENT_PROFILE;
  const ready = plan !== undefined && newName.trim() !== "";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={dialogShellClass("sm")}
        data-testid="profile-rename-dialog"
        showCloseButton={false}
      >
        <EntityDialogHeader eyebrow="Profiles" icon={TextCursorInput} title={`Rename ${profile}`} />
        <EntityDialogBody>
          {permanent ? (
            <p className="text-small-body text-muted" data-testid="profile-rename-permanent">
              {PROFILE_PERMANENT_LINE}
            </p>
          ) : (
            <div className="flex flex-col gap-3">
              <Field data-invalid={error !== null ? true : undefined}>
                <FieldLabel htmlFor="profile-rename-name">New name</FieldLabel>
                <Input
                  id="profile-rename-name"
                  value={newName}
                  onChange={event => {
                    setTouched(true);
                    onNewNameChange(event.target.value);
                  }}
                  aria-invalid={error !== null}
                  data-testid="profile-rename-name-input"
                />
                {error !== null ? <FieldError>{error}</FieldError> : null}
              </Field>
              {planLoading && touched ? (
                <div className="flex items-center gap-2 text-small-body text-subtle">
                  <Spinner className="size-3.5" />
                  <span>Checking what this will change…</span>
                </div>
              ) : null}
              {plan !== undefined ? (
                <ProfileRenamePlan
                  plan={plan}
                  newName={newName}
                  acceptedRepos={acceptedRepos}
                  onToggleRepo={onToggleRepo}
                />
              ) : null}
            </div>
          )}
        </EntityDialogBody>
        {permanent ? (
          <EntityDialogFooter
            cancelLabel="Close"
            cancelTestId="profile-rename-cancel"
            onCancel={() => onOpenChange(false)}
            primaryLabel="Rename"
            primaryDisabled
            primaryTestId="profile-rename-confirm"
          />
        ) : (
          <EntityDialogFooter
            cancelTestId="profile-rename-cancel"
            isSaving={isPending}
            onCancel={() => onOpenChange(false)}
            onPrimary={() => {
              if (plan !== undefined) onRename(plan.revision);
            }}
            primaryDisabled={!ready || planLoading}
            primaryLabel="Rename"
            primaryTestId="profile-rename-confirm"
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
