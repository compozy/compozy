import {
  ROLE_ORDER,
  RoleList,
  SettingsPageFrame,
  SettingsPageState,
  SettingsSaveBar,
  useSettingsSaveBarState,
  useSettingsRolesPage,
  useSettingsTopbar,
} from "@/systems/settings";
import { SkeletonRows } from "@compozy/ui";

const TEST_PREFIX = "settings-page-roles";

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
      <div className="mx-auto flex w-full max-w-settings-page-wide flex-col px-6 pt-5">
        <div
          aria-busy="true"
          className="rounded-lg bg-canvas shadow-card px-4"
          data-testid={`${TEST_PREFIX}-loading`}
          role="status"
        >
          <SkeletonRows
            count={ROLE_ORDER.length}
            rowClassName="border-t border-line-soft py-3 first:border-t-0"
          />
          <span className="sr-only">Loading roles</span>
        </div>
      </div>
    );
  }

  if (page.error) {
    return (
      <SettingsPageState error={page.error} onRetry={page.handleRetry} slug="roles" state="error" />
    );
  }

  // Empty projection is a protocol anomaly, not a normal empty state.
  if (page.isEmpty) {
    return (
      <div className="flex flex-1" data-testid={`${TEST_PREFIX}-empty`}>
        <SettingsPageState
          error="No roles were returned."
          onRetry={page.handleRetry}
          slug="roles"
          state="error"
        />
      </div>
    );
  }

  return (
    <SettingsPageFrame
      slug="roles"
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
