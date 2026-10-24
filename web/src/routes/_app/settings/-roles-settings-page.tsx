import {
  RoleList,
  SettingsPageFrame,
  SettingsPageState,
  SettingsSaveBar,
  useSettingsSaveBarState,
  useSettingsRolesPage,
  useSettingsTopbar,
} from "@/systems/settings";

const TEST_PREFIX = "settings-page-roles";

export function RolesSettingsPage() {
  const page = useSettingsRolesPage();
  useSettingsTopbar();
  const saveBarState = useSettingsSaveBarState({
    isDirty: page.isDirty,
    isInvalid: page.isInvalid,
    isSaving: page.isSaving,
    error: page.saveError,
    warnings: page.warnings,
    lastAppliedLabel: page.lastAppliedLabel,
  });

  if (page.isLoading) {
    return <SettingsPageState slug="roles" state="loading" />;
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
