import type { ProfileRow } from "../lib/profile-rows";
import { ProfileGlyph } from "./profile-glyph";

export interface ProfilePaletteRowProps {
  row: ProfileRow;
  /** What Enter will do — "current", "unarchive", or empty. */
  meta?: string;
}

/**
 * One profile in the command palette.
 *
 * An unavailable row keeps full-contrast text and carries its typed reason in a
 * structured slot, so it stays readable and honest about why rather than fading
 * into decoration.
 */
export function ProfilePaletteRow({ row, meta }: ProfilePaletteRowProps) {
  return (
    <span
      className="flex min-w-0 flex-1 items-center gap-2"
      data-slot="profile-palette-row"
      data-profile={row.name}
    >
      <ProfileGlyph
        decorative
        size="sm"
        name={row.name}
        color={row.color}
        current={row.current}
        needsSetup={row.needsSetup}
      />
      <span className="truncate">{row.name}</span>
      {row.disabledReason !== "" ? (
        <span
          data-slot="os-palette-reason"
          className="ml-auto shrink-0 text-micro font-medium text-warning"
        >
          {row.disabledReason}
        </span>
      ) : meta ? (
        <span className="ml-auto shrink-0 text-micro text-subtle">{meta}</span>
      ) : null}
    </span>
  );
}
