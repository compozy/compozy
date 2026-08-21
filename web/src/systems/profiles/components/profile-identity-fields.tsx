import { Field, FieldError, FieldLabel, Input, SymbolPicker, type SymbolValue } from "@compozy/ui";

import {
  PROFILE_EMOJI_OPTIONS,
  PROFILE_ICON_OPTIONS,
  PROFILE_ICON_REGISTRY,
  PROFILE_IDENTITY_SWATCHES,
} from "../lib/profile-identity";
import { ProfileGlyph } from "./profile-glyph";

export interface ProfileIdentityFieldsProps {
  name: string;
  onNameChange?: (next: string) => void;
  color: string;
  onColorChange: (next: string) => void;
  onColorValidityChange?: (valid: boolean) => void;
  symbol: SymbolValue;
  onSymbolChange: (next: SymbolValue) => void;
  nameError?: string | null;
  /** Rename and archived edits keep the picker but drop the name field. */
  showName?: boolean;
  nameLabel?: string;
  testIdPrefix: string;
}

/** Name plus the symbol picker — the shared identity half of create and edit. */
export function ProfileIdentityFields({
  name,
  onNameChange,
  color,
  onColorChange,
  onColorValidityChange,
  symbol,
  onSymbolChange,
  nameError = null,
  showName = true,
  nameLabel = "Name",
  testIdPrefix,
}: ProfileIdentityFieldsProps) {
  return (
    <div className="flex flex-col gap-3">
      {showName ? (
        <Field data-invalid={nameError !== null ? true : undefined}>
          <FieldLabel htmlFor={`${testIdPrefix}-name`}>{nameLabel}</FieldLabel>
          <div className="flex items-center gap-2.5">
            <ProfileGlyph
              size="lg"
              name={name || "New profile"}
              color={color}
              {...(symbol.kind === "emoji" ? { emoji: symbol.value } : { icon: symbol.value })}
            />
            <Input
              id={`${testIdPrefix}-name`}
              value={name}
              onChange={event => onNameChange?.(event.target.value)}
              placeholder="Profile name"
              aria-invalid={nameError !== null}
              data-testid={`${testIdPrefix}-name-input`}
            />
          </div>
          {nameError !== null ? <FieldError>{nameError}</FieldError> : null}
        </Field>
      ) : null}
      <SymbolPicker
        color={color}
        onColorChange={onColorChange}
        onColorValidityChange={onColorValidityChange}
        symbol={symbol}
        onSymbolChange={onSymbolChange}
        icons={PROFILE_ICON_OPTIONS}
        iconRegistry={PROFILE_ICON_REGISTRY}
        emojis={PROFILE_EMOJI_OPTIONS}
        swatches={PROFILE_IDENTITY_SWATCHES}
        data-testid={`${testIdPrefix}-symbol-picker`}
      />
    </div>
  );
}
