import {
  Alert,
  AlertDescription,
  Button,
  NativeSelect,
  NativeSelectOption,
  Switch,
} from "@compozy/ui";

import {
  AttentionSystemStateChip,
  SettingsGroup,
  SettingsPageFrame,
  SettingsPageState,
  SettingRow,
  attentionSystemStateNote,
  useSettingsAttentionPage,
  useSettingsTopbar,
} from "@/systems/settings";
import { useActiveWorkspace } from "@/systems/workspace";

function MutedWorkspaces({
  page,
  workspaces,
}: {
  page: ReturnType<typeof useSettingsAttentionPage>;
  workspaces: ReadonlyArray<{ id: string; name: string }>;
}) {
  const muted = page.config?.muted_workspaces ?? [];
  const mutedIds = new Set(muted);
  const available = workspaces.filter(workspace => !mutedIds.has(workspace.id));
  return (
    <SettingsGroup title="Muted projects">
      <SettingRow
        label="Mute a project"
        description="You won't be alerted; they still appear in the bell."
        control={
          <NativeSelect
            aria-label="Mute a project"
            data-testid="settings-attention-mute-picker"
            size="sm"
            value=""
            disabled={page.isSaving || available.length === 0}
            onChange={event => page.muteWorkspace(event.target.value)}
          >
            <NativeSelectOption value="">Choose a project…</NativeSelectOption>
            {available.map(workspace => (
              <NativeSelectOption key={workspace.id} value={workspace.id}>
                {workspace.name}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        }
      />
      {muted.length === 0 ? (
        <p className="px-4 py-3 text-form-label text-subtle">No muted projects.</p>
      ) : (
        muted.map(workspaceId => (
          <SettingRow
            key={workspaceId}
            label={workspaces.find(entry => entry.id === workspaceId)?.name ?? workspaceId}
            control={
              <Button
                variant="ghost"
                size="sm"
                disabled={page.isSaving}
                data-testid={`settings-attention-unmute-${workspaceId}`}
                onClick={() => page.unmuteWorkspace(workspaceId)}
              >
                Unmute
              </Button>
            }
          />
        ))
      )}
    </SettingsGroup>
  );
}

export function AttentionSettingsPage() {
  const page = useSettingsAttentionPage();
  useSettingsTopbar();
  const { workspaces } = useActiveWorkspace();

  if (page.isLoading) {
    return <SettingsPageState slug="attention" state="loading" />;
  }

  if (page.error || page.config === null) {
    return (
      <SettingsPageState
        error={page.error}
        onRetry={page.handleRetry}
        slug="attention"
        state="error"
      />
    );
  }

  return (
    <SettingsPageFrame
      description="Changes apply immediately."
      restart={page.restart}
      slug="attention"
    >
      {page.saveError ? (
        <Alert data-testid="settings-attention-save-error" role="alert" variant="danger">
          <AlertDescription>{page.saveError}</AlertDescription>
        </Alert>
      ) : null}
      <SettingsGroup title="Delivery">
        <SettingRow
          label="Pop-up alerts"
          control={
            <Switch
              checked={page.config.toasts}
              disabled={page.isSaving}
              aria-label="Pop-up alerts"
              data-testid="settings-attention-toasts"
              onCheckedChange={page.setToasts}
            />
          }
        />
        <SettingRow
          label="Sound"
          control={
            <Switch
              checked={page.config.sound}
              disabled={page.isSaving}
              aria-label="Sound"
              data-testid="settings-attention-sound"
              onCheckedChange={page.setSound}
            />
          }
        />
        <SettingRow
          label="System notifications"
          description={attentionSystemStateNote(page.systemState)}
          control={
            <span className="flex items-center gap-2">
              <AttentionSystemStateChip state={page.systemState} />
              <Switch
                checked={page.config.system && page.systemState === "granted"}
                disabled={page.isSaving || page.systemState === "unsupported"}
                aria-label="System notifications"
                data-testid="settings-attention-system"
                onCheckedChange={page.setSystem}
              />
            </span>
          }
        />
      </SettingsGroup>
      <MutedWorkspaces page={page} workspaces={workspaces} />
    </SettingsPageFrame>
  );
}
