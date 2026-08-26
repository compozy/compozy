import { Field, FieldError, FieldLabel, Input, SymbolPicker, type SymbolValue } from "@compozy/ui";

import type { ProfileIconCatalogViewModel } from "../hooks/use-profile-icon-catalog";
import {
  PROFILE_EMOJIBASE_URL,
  PROFILE_IDENTITY_SWATCHES,
  PROFILE_SPRITE_URL,
} from "../lib/profile-identity";
import { ProfileGlyph } from "./profile-glyph";

interface ProfileIdentityFieldsBaseProps {
  catalog: ProfileIconCatalogViewModel;
  name: string;
  color: string;
  onColorChange: (next: string) => void;
  onColorValidityChange?: (valid: boolean) => void;
  symbol: SymbolValue;
  onSymbolChange: (next: SymbolValue) => void;
  nameError?: string | null;
  nameLabel?: string;
  testIdPrefix: string;
}

export type ProfileIdentityFieldsProps = ProfileIdentityFieldsBaseProps &
  (
    | {
        /** Create renders an editable profile name. */
        showName?: true;
        onNameChange: (next: string) => void;
      }
    | {
        /** Identity edits keep the picker but drop the immutable name field. */
        showName: false;
        onNameChange?: never;
      }
  );

/** Name plus the symbol picker — the shared identity half of create and edit. */
export function ProfileIdentityFields(props: ProfileIdentityFieldsProps) {
  const {
    name,
    color,
    onColorChange,
    onColorValidityChange,
    symbol,
    onSymbolChange,
    nameError = null,
    nameLabel = "Name",
    testIdPrefix,
    catalog,
  } = props;
  return (
    <div className="flex flex-col gap-3">
      {props.showName !== false ? (
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
              onChange={event => props.onNameChange(event.target.value)}
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
        icons={catalog.icons}
        iconsLoading={catalog.loading}
        spriteUrl={PROFILE_SPRITE_URL}
        emojibaseUrl={PROFILE_EMOJIBASE_URL}
        swatches={PROFILE_IDENTITY_SWATCHES}
        data-testid={`${testIdPrefix}-symbol-picker`}
      />
    </div>
  );
}
