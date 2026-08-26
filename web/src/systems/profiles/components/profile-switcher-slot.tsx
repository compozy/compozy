import { useProfileLens } from "../hooks/use-profile-lens";
import { useProfileSwitcher } from "../hooks/use-profile-switcher";
import { openProfileDialog } from "../stores/profile-dialog-store";
import { ProfileSwitcher } from "./profile-switcher";

export interface ProfileSwitcherSlotProps {
  /** Opens Settings → Profiles; the shell owns how a window gets raised. */
  onOpenSettings: () => void;
}

/**
 * The menubar switcher, connected.
 *
 * The bar itself stays presentational: it offers a slot, and this resolves the
 * lens, the rows, and the switch against the canonical selection routes.
 */
export function ProfileSwitcherSlot({ onOpenSettings }: ProfileSwitcherSlotProps) {
  const lens = useProfileLens();
  const model = useProfileSwitcher(lens);
  return (
    <ProfileSwitcher
      rows={model.rows}
      activeName={model.activeName}
      aggregate={model.aggregate}
      quiet={model.quiet}
      archivedCount={model.archivedCount}
      onSelectProfile={model.selectProfile}
      onSelectAggregate={model.selectAggregate}
      onCreate={model.create}
      onEditProfile={name => openProfileDialog({ flow: "update", profile: name })}
      onOpenSettings={onOpenSettings}
      manageable={model.manageable}
      isLoading={model.isLoading}
      error={model.error}
      onRetry={model.retry}
    />
  );
}
