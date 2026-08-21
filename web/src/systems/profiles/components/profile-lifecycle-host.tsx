import { useProfileEventStream } from "../hooks/use-profile-event-stream";
import { useProfileLens } from "../hooks/use-profile-lens";
import { useProfiles } from "../hooks/use-profiles";
import { ProfileLifecycleDialogs } from "./profile-lifecycle-dialogs";

/**
 * Shell-level owner of the profile dialogs and the live event stream.
 *
 * Mounted once so a lifecycle flow started from the command palette does not
 * depend on Settings being open, and so the remembered-choice projection stays
 * current whether the switch happened here, in a terminal, or in another
 * browser.
 */
export function ProfileLifecycleHost() {
  const lens = useProfileLens();
  const profiles = useProfiles();
  useProfileEventStream();
  return <ProfileLifecycleDialogs profiles={profiles.data ?? []} lens={lens} />;
}
