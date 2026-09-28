import { useSettingsHooksPage } from "@/systems/settings/hooks/use-settings-hooks-page";
import { SettingsPageFrame, SettingsPageState, useSettingsTopbar } from "@/systems/settings";

import { HooksSection } from "./-hooks-section";

export function HooksSettingsPage() {
  const page = useSettingsHooksPage();
  useSettingsTopbar("hooks");
  if (page.isLoading) return <SettingsPageState slug="hooks" state="loading" />;
  if (page.error || !page.envelope)
    return (
      <SettingsPageState error={page.error} onRetry={page.handleRetry} slug="hooks" state="error" />
    );
  return (
    <SettingsPageFrame
      meta={[
        {
          key: "hooks",
          content: (
            <span>
              <span className="font-medium text-muted">{page.hooksCounts.enabled}</span> of{" "}
              {page.hooksCounts.total} hooks enabled
            </span>
          ),
        },
      ]}
      restart={page.restart}
      slug="hooks"
    >
      <HooksSection
        canMutate={page.canMutateHooks}
        hookError={page.hookError}
        hooks={page.hooks}
        onToggle={page.toggleHookEnabled}
        pendingHookName={page.pendingHookName}
      />
    </SettingsPageFrame>
  );
}
