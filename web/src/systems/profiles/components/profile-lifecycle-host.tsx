import { useProfileEventStream } from "../hooks/use-profile-event-stream";
import { useProfiles } from "../hooks/use-profiles";
import { ProfileLifecycleDialogs } from "./profile-lifecycle-dialogs";

/**
 * Shell-level owner of the profile dialogs and the live event stream.
 *
 * Mounted once so a lifecycle flow started from the command palette does not
 * depend on Settings being open, and so the remembered-choice projection stays
 * current whether the switch happened here, in a terminal, or in another
 * browser.
 *
 * The global feed must survive desktop reconnection so deleting the viewed
 * profile can recover the view to the default profile.
 */
export function ProfileLifecycleHost({
  onSetAutomationEnabled,
  onProfileRenamed,
}: {
  onSetAutomationEnabled: (identity: string, profile: string, enabled: boolean) => Promise<void>;
  /**
   * A rename relabels rows that other systems cache under keys holding no profile
   * name, such as the All profiles catalog. This system cannot import those
   * systems, so the shell supplies the reread. It runs for a rename made here and
   * for one announced by the event stream (#1220).
   */
  onProfileRenamed: () => Promise<unknown>;
}) {
  const profiles = useProfiles();
  useProfileEventStream({ onProfileRenamed });
  return (
    <ProfileLifecycleDialogs
      profiles={profiles.data ?? []}
      onSetAutomationEnabled={onSetAutomationEnabled}
      onProfileRenamed={onProfileRenamed}
    />
  );
}
