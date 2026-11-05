import { useSettingsPage } from "../hooks/use-settings-page";
import { SettingsRestartNotice } from "./settings-restart-notice";

/**
 * The restart notice of one settings section for a host outside the Settings window (the
 * Marketplace MCPs view), so a save that needs a daemon restart can be finished in place.
 */
export function SettingsPageRestartNotice({ slug }: { slug: string }) {
  const page = useSettingsPage({ currentSlug: slug });
  return <SettingsRestartNotice restart={page.restart} slug={slug} />;
}
