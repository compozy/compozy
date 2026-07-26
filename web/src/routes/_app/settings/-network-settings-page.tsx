import { AlertCircle } from "lucide-react";
import { useState } from "react";

import { useSettingsNetworkPage } from "@/systems/settings/hooks/use-settings-network-page";
import {
  NetworkSettingsSections,
  SettingsPageFrame,
  SettingsSaveBar,
  useSettingsSaveBarState,
  useSettingsTopbar,
} from "@/systems/settings";
import { Button, Spinner } from "@agh/ui";

export function NetworkSettingsPage() {
  const page = useSettingsNetworkPage();
  useSettingsTopbar("network");
  const [validationErrors, setValidationErrors] = useState<Record<string, string | null>>({});
  const setValidationError = (key: string) => (message: string | null) => {
    setValidationErrors(current =>
      current[key] === message ? current : { ...current, [key]: message }
    );
  };
  const isInvalid = Object.values(validationErrors).some(message => message !== null);
  const saveBarState = useSettingsSaveBarState({
    isDirty: page.isDirty,
    isInvalid,
    isSaving: page.isSaving,
    error: page.saveError,
    warnings: page.warnings,
    lastAppliedLabel: page.lastAppliedLabel,
  });
  const runtime = page.envelope?.runtime;

  if (page.isLoading) {
    return (
      <div
        aria-label="Loading network settings"
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-network-loading"
        role="status"
      >
        <Spinner aria-hidden="true" className="size-5 text-subtle" />
      </div>
    );
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-network-error"
      >
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle className="size-6 text-danger" />
          <p className="text-sm text-subtle">
            {page.error?.message ?? "Failed to load network settings"}
          </p>
          <Button onClick={page.handleRetry} size="sm" type="button" variant="outline">
            Retry
          </Button>
        </div>
      </div>
    );
  }

  if (!runtime) {
    return null;
  }

  return (
    <SettingsPageFrame
      description="How this daemon talks to other agents over AGH Network."
      meta={[
        {
          key: "status",
          content: runtime.available ? (
            <span>{runtime.status ?? (runtime.enabled ? "ready" : "disabled")}</span>
          ) : (
            <span>runtime unavailable</span>
          ),
        },
        ...(runtime.available
          ? [
              {
                key: "participants",
                content: (
                  <span>
                    <span className="font-medium text-muted">{runtime.local_peers}</span> live
                    participants
                  </span>
                ),
              },
            ]
          : []),
      ]}
      restart={page.restart}
      saveBar={
        <SettingsSaveBar
          slug="network"
          state={saveBarState}
          onSave={page.handleSave}
          onReset={page.handleReset}
        />
      }
      slug="network"
    >
      <NetworkSettingsSections
        runtime={runtime}
        draft={page.draft}
        setDraft={page.setDraft}
        validationErrors={validationErrors}
        setValidationError={setValidationError}
      />
    </SettingsPageFrame>
  );
}
