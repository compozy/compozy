import { useState } from "react";
import { Palette } from "lucide-react";

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

import { symbolOf } from "../lib/profile-identity";
import type { ProfilePayload, UpdateProfileParams } from "../types";
import { ProfileIdentityFields } from "./profile-identity-fields";

export interface ProfileIdentityDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  profile: ProfilePayload;
  isPending: boolean;
  error?: string | null;
  onSave: (patch: UpdateProfileParams) => void;
}

export function ProfileIdentityDialog({
  open,
  onOpenChange,
  profile,
  isPending,
  error = null,
  onSave,
}: ProfileIdentityDialogProps) {
  const [color, setColor] = useState(profile.color);
  const [symbol, setSymbol] = useState<SymbolValue>(() => symbolOf(profile));
  const [colorValid, setColorValid] = useState(true);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={dialogShellClass("sm")}
        data-testid="profile-identity-dialog"
        showCloseButton={false}
      >
        <EntityDialogHeader eyebrow="Profiles" icon={Palette} title={`Edit ${profile.name}`} />
        <EntityDialogBody>
          <ProfileIdentityFields
            name={profile.name}
            color={color}
            onColorChange={setColor}
            onColorValidityChange={setColorValid}
            symbol={symbol}
            onSymbolChange={setSymbol}
            showName={false}
            testIdPrefix="profile-identity"
          />
          {error !== null ? <FieldError>{error}</FieldError> : null}
        </EntityDialogBody>
        <EntityDialogFooter
          cancelTestId="profile-identity-cancel"
          isSaving={isPending}
          onCancel={() => onOpenChange(false)}
          onPrimary={() => {
            if (!colorValid) return;
            onSave({
              color,
              ...(symbol.kind === "emoji" ? { emoji: symbol.value } : { icon: symbol.value }),
            });
          }}
          primaryDisabled={!colorValid}
          primaryLabel="Save"
          primaryTestId="profile-identity-confirm"
        />
      </DialogContent>
    </Dialog>
  );
}
