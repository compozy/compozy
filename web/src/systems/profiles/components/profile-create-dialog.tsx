import { useState } from "react";
import { UserRound } from "lucide-react";

import {
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  FieldError,
  type SymbolValue,
} from "@compozy/ui";

import { NAME_REQUIRED_MESSAGE, PROFILE_SEPARATION_LINE } from "../lib/profile-copy";
import { starterIdentity } from "../lib/profile-identity";
import type { ProfileIconCatalogViewModel } from "../hooks/use-profile-icon-catalog";
import type { ProfileLens } from "../types";
import { ProfileIdentityFields } from "./profile-identity-fields";

export interface ProfileCreateDialogProps {
  catalog: ProfileIconCatalogViewModel;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Drives the starter identity so two profiles made back to back differ. */
  existingCount: number;
  lens: ProfileLens;
  isPending: boolean;
  /** Name refusal from the daemon, tied to the value it rejected. */
  nameError?: { name: string; message: string } | null;
  error?: string | null;
  initialName?: string;
  onCreate: (input: {
    name: string;
    color: string;
    symbol: SymbolValue;
    activate: { scope: string; profile: string; workspace_id?: string };
  }) => void;
}

export function ProfileCreateDialog({
  catalog,
  open,
  onOpenChange,
  existingCount,
  lens,
  isPending,
  nameError = null,
  error = null,
  initialName = "",
  onCreate,
}: ProfileCreateDialogProps) {
  const [name, setName] = useState(initialName);
  const [identity, setIdentity] = useState<{
    color: string;
    symbol: SymbolValue;
    colorValid: boolean;
  }>(() => {
    const starter = starterIdentity(existingCount);
    return {
      color: starter.color,
      symbol: { kind: "icon", value: starter.icon },
      colorValid: true,
    };
  });
  const [localError, setLocalError] = useState<string | null>(null);
  const currentNameError =
    nameError?.name === name.trim() ? nameError.message : name.trim() === "" ? localError : null;

  const submit = () => {
    if (!identity.colorValid) return;
    const trimmed = name.trim();
    if (trimmed === "") {
      setLocalError(NAME_REQUIRED_MESSAGE);
      return;
    }
    setLocalError(null);
    onCreate({
      name: trimmed,
      color: identity.color,
      symbol: identity.symbol,
      activate: {
        scope: lens.scope,
        profile: trimmed,
        ...(lens.scope === "workspace" ? { workspace_id: lens.workspaceId } : {}),
      },
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={dialogShellClass("sm")}
        data-testid="profile-create-dialog"
        showCloseButton={false}
      >
        <EntityDialogHeader
          eyebrow="Profiles"
          icon={UserRound}
          onClose={isPending ? undefined : () => onOpenChange(false)}
          title="Create profile"
        />
        <EntityDialogBody>
          <ProfileIdentityFields
            catalog={catalog}
            name={name}
            onNameChange={setName}
            color={identity.color}
            onColorChange={color => setIdentity(current => ({ ...current, color }))}
            onColorValidityChange={colorValid =>
              setIdentity(current => ({ ...current, colorValid }))
            }
            symbol={identity.symbol}
            onSymbolChange={symbol => setIdentity(current => ({ ...current, symbol }))}
            nameError={currentNameError}
            testIdPrefix="profile-create"
          />
          {error !== null ? <FieldError>{error}</FieldError> : null}
        </EntityDialogBody>
        <EntityDialogFooter
          cancelTestId="profile-create-cancel"
          hint={PROFILE_SEPARATION_LINE}
          isSaving={isPending}
          onCancel={() => onOpenChange(false)}
          onPrimary={submit}
          primaryLabel="Create"
          primaryDisabled={!identity.colorValid}
          primaryTestId="profile-create-confirm"
        />
      </DialogContent>
    </Dialog>
  );
}
