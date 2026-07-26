import { AlertCircle } from "lucide-react";

import { ROLE_ORDER } from "@/systems/settings";

import {
  RoleList,
  SettingsPageFrame,
  SettingsSaveBar,
  useSettingsSaveBarState,
  useSettingsRolesPage,
  useSettingsTopbar,
} from "@/systems/settings";
import { Button, Skeleton } from "@agh/ui";

const TEST_PREFIX = "settings-page-roles";

function RolesNotice({
  message,
  onRetry,
  testId,
}: {
  message: string;
  onRetry: () => void;
  testId: string;
}) {
  return (
    <div className="flex flex-1 items-center justify-center" data-testid={testId}>
      <div className="flex flex-col items-center gap-2 text-center">
        <AlertCircle className="size-6 text-danger" />
        <p className="max-w-settings-page-description text-sm text-subtle">{message}</p>
        <Button onClick={onRetry} size="sm" type="button" variant="outline">
          Retry
        </Button>
      </div>
    </div>
  );
}

export function RolesSettingsPage() {
  const page = useSettingsRolesPage();
  useSettingsTopbar("roles");
  const saveBarState = useSettingsSaveBarState({
    isDirty: page.isDirty,
    isInvalid: page.isInvalid,
    isSaving: page.isSaving,
    error: page.saveError,
    warnings: page.warnings,
    lastAppliedLabel: page.lastAppliedLabel,
  });

  if (page.isLoading) {
    return (
      <div className="mx-auto flex w-full max-w-settings-page-wide flex-col gap-2.5 px-6 pt-5">
        <Skeleton className="h-4 w-24" />
        <div
          className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
          data-testid={`${TEST_PREFIX}-loading`}
        >
          {ROLE_ORDER.map(role => (
            <div
              key={role}
              className="flex items-center justify-between gap-3 px-4 py-3 not-last:border-b not-last:border-line-soft"
            >
              <div className="flex min-w-0 flex-1 flex-col gap-1.5">
                <Skeleton className="h-3.5 w-40" />
                <Skeleton className="h-3 w-3/5" />
              </div>
              <Skeleton className="h-5 w-9 rounded-full" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (page.error) {
    return (
      <RolesNotice
        message={page.error.message}
        onRetry={page.handleRetry}
        testId={`${TEST_PREFIX}-error`}
      />
    );
  }

  // Empty projection is a protocol anomaly, not a normal empty state.
  if (page.isEmpty) {
    return (
      <RolesNotice
        message="Roles unavailable — no role projection was returned."
        onRetry={page.handleRetry}
        testId={`${TEST_PREFIX}-empty`}
      />
    );
  }

  return (
    <SettingsPageFrame
      slug="roles"
      description="Choose how each background role is routed."
      restart={page.restart}
      saveBar={
        <SettingsSaveBar
          slug="roles"
          state={saveBarState}
          onSave={page.handleSave}
          onReset={page.handleReset}
        />
      }
      width="wide"
    >
      <RoleList
        roles={page.roles}
        options={page.runtimeOptions}
        disclosure={page.disclosure}
        validationErrors={page.validationErrors}
        disabled={page.isSaving}
        draftRevision={page.draftRevision}
        setRoleEnabled={page.setRoleEnabled}
        setRoleAgent={page.setRoleAgent}
        setRoleField={page.setRoleField}
        setRoleRuntime={page.setRoleRuntime}
        clearRuntime={page.clearRuntime}
        setNumberFieldValidity={page.setNumberFieldValidity}
        addFallback={page.addFallback}
        removeFallback={page.removeFallback}
        updateFallback={page.updateFallback}
        registerFieldRef={page.registerFieldRef}
      />
    </SettingsPageFrame>
  );
}
