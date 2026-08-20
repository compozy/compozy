import { AlertCircle } from "lucide-react";

import { Alert, AlertDescription, Button, Spinner, Switch } from "@compozy/ui";

import {
  AttentionSystemStateChip,
  SettingsGroup,
  SettingsPageFrame,
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
    <SettingsGroup title="Muted workspaces">
      <SettingRow
        label="Silence a workspace"
        description="Silenced everywhere; bell rows and counts remain"
        control={
          <select
            aria-label="Mute a workspace"
            data-testid="settings-attention-mute-picker"
            className="h-7 rounded-sm border border-line bg-canvas px-2 text-small-body text-fg"
            value=""
            disabled={page.isSaving || available.length === 0}
            onChange={event => page.muteWorkspace(event.target.value)}
          >
            <option value="">Mute a workspace…</option>
            {available.map(workspace => (
              <option key={workspace.id} value={workspace.id}>
                {workspace.name}
              </option>
            ))}
          </select>
        }
      />
      {muted.length === 0 ? (
        <SettingRow label="Nothing muted" control={null} />
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
  useSettingsTopbar("attention");
  const { workspaces } = useActiveWorkspace();

  if (page.isLoading) {
    return (
      <div
        aria-label="Loading attention settings"
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-attention-loading"
        role="status"
      >
        <Spinner aria-hidden="true" className="size-5 text-subtle" />
      </div>
    );
  }

  if (page.error || page.config === null) {
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-attention-error"
      >
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle className="size-6 text-danger" />
          <p className="text-sm text-subtle">
            {page.error?.message ?? "Failed to load attention settings"}
          </p>
          <Button onClick={page.handleRetry} size="sm" type="button" variant="outline">
            Retry
          </Button>
        </div>
      </div>
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
          label="Toasts"
          control={
            <Switch
              checked={page.config.toasts}
              disabled={page.isSaving}
              aria-label="Toasts"
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
