import { AlertCircle, Check, Minus, TriangleAlert } from "lucide-react";

import { Alert, AlertDescription, Button, Icon, Spinner, Switch } from "@compozy/ui";

import {
  SettingsGroup,
  SettingsPageFrame,
  SettingRow,
  useSettingsAttentionPage,
  useSettingsTopbar,
} from "@/systems/settings";
import type { SystemNotificationState } from "@/systems/os";
import { useActiveWorkspace } from "@/systems/workspace";

const SYSTEM_CHIP: Record<
  SystemNotificationState,
  { label: string; icon: typeof Check; className: string; note: string }
> = {
  granted: {
    label: "Armed",
    icon: Check,
    className: "bg-success-tint text-success",
    note: "Only while the app is in the background",
  },
  denied: {
    label: "Denied",
    icon: TriangleAlert,
    className: "bg-warning-tint text-warning",
    note: "Permission denied — allow Compozy in your system notification settings",
  },
  unsupported: {
    label: "Unavailable",
    icon: Minus,
    className: "bg-neutral-tint text-muted",
    note: "Not available in this browser",
  },
  default: {
    label: "Not armed",
    icon: Minus,
    className: "bg-neutral-tint text-muted",
    note: "Only while the app is in the background",
  },
};

function SystemStateChip({ state }: { state: SystemNotificationState }) {
  const chip = SYSTEM_CHIP[state];
  return (
    <span
      data-testid={`settings-attention-system-${state}`}
      className={`inline-flex h-5 items-center gap-1.5 rounded-full px-2 text-micro font-semibold ${chip.className}`}
    >
      <Icon as={chip.icon} size="xs" />
      {chip.label}
    </span>
  );
}

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

  const systemChip = SYSTEM_CHIP[page.systemState];
  return (
    <SettingsPageFrame
      description="How CompozyOS tells you a session needs you. Changes apply immediately."
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
          description="In-app, for needs-you moments and finished sessions"
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
          description="One chime per delivery, following the same rules"
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
          description={systemChip.note}
          control={
            <span className="flex items-center gap-2">
              <SystemStateChip state={page.systemState} />
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
