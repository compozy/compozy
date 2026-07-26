import { Bell, Plus, Trash2 } from "lucide-react";
import { useState } from "react";

import type { CreateNotificationPresetRequest, NotificationPresetEntry } from "../types";
import {
  Button,
  Empty,
  Input,
  Pill,
  Section,
  SkeletonRows,
  Spinner,
  Switch,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@agh/ui";

interface NotificationPresetsPanelProps {
  presets: NotificationPresetEntry[];
  isLoading: boolean;
  error: string | null;
  pendingName: string | null;
  canMutate: boolean;
  onCreate: (body: CreateNotificationPresetRequest) => void;
  onToggle: (preset: NotificationPresetEntry, nextEnabled: boolean) => void;
  onDelete: (preset: NotificationPresetEntry) => void;
}

/**
 * Notification presets as a registry list: bell-glyph rows with origin tags
 * and an "events → target" meta line; the create form stays behind the New
 * preset toggle so the registry reads first.
 */
export function NotificationPresetsPanel({
  presets,
  isLoading,
  error,
  pendingName,
  canMutate,
  onCreate,
  onToggle,
  onDelete,
}: NotificationPresetsPanelProps) {
  const [createOpen, setCreateOpen] = useState(false);
  const [form, setForm] = useState({
    name: "",
    events: "task.run_*",
    target: "",
    filter: "",
    enabled: false,
  });
  const [localError, setLocalError] = useState<string | null>(null);
  const createPending = pendingName !== null;
  const formErrorId = localError ? "notification-preset-create-error" : undefined;

  const submit = () => {
    if (createPending) return;
    const eventList = form.events.split(",").flatMap(item => {
      const event = item.trim();
      return event ? [event] : [];
    });
    const targets = parseNotificationPresetTargetEntry(form.target);
    if (form.name.trim() === "" || eventList.length === 0) {
      setLocalError("Name and event are required.");
      return;
    }
    if (targets === null) {
      setLocalError("Target must use bridge_id:canonical_route.");
      return;
    }
    setLocalError(null);
    onCreate({
      name: form.name.trim(),
      events: eventList,
      targets,
      filter: form.filter.trim(),
      enabled: form.enabled,
    });
  };

  return (
    <Section
      data-testid="settings-page-hooks-notification-presets-section"
      label="Notifications"
      note="Presets apply immediately — no restart."
      right={
        <Button
          aria-expanded={createOpen}
          data-testid="settings-page-hooks-notification-preset-new"
          disabled={!canMutate || createPending}
          onClick={() => setCreateOpen(open => !open)}
          size="sm"
          type="button"
          variant="neutral"
        >
          <Plus className="size-3.5" />
          New preset
        </Button>
      }
    >
      <div className="flex flex-col gap-3">
        {createOpen ? (
          <div
            className="flex flex-col gap-2 rounded-md border border-line bg-canvas px-3 py-3"
            data-testid="settings-page-hooks-notification-preset-createform"
          >
            <div className="grid gap-2 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)_minmax(0,1.2fr)_minmax(0,1fr)]">
              <Input
                aria-describedby={formErrorId}
                aria-invalid={Boolean(localError)}
                aria-label="Notification preset name"
                className="font-mono"
                data-testid="settings-page-hooks-notification-preset-name"
                placeholder="custom_alert"
                disabled={createPending || !canMutate}
                value={form.name}
                onChange={event => setForm(current => ({ ...current, name: event.target.value }))}
              />
              <Input
                aria-describedby={formErrorId}
                aria-invalid={Boolean(localError)}
                aria-label="Notification preset events"
                className="font-mono"
                data-testid="settings-page-hooks-notification-preset-events"
                disabled={createPending || !canMutate}
                value={form.events}
                onChange={event => setForm(current => ({ ...current, events: event.target.value }))}
              />
              <Input
                aria-describedby={formErrorId}
                aria-invalid={Boolean(localError)}
                aria-label="Notification preset target"
                className="font-mono"
                data-testid="settings-page-hooks-notification-preset-target"
                disabled={createPending || !canMutate}
                placeholder="bridge_slack_ops:channel:ops"
                value={form.target}
                onChange={event => setForm(current => ({ ...current, target: event.target.value }))}
              />
              <Input
                aria-describedby={formErrorId}
                aria-invalid={Boolean(localError)}
                aria-label="Notification preset filter"
                className="font-mono"
                data-testid="settings-page-hooks-notification-preset-filter"
                disabled={createPending || !canMutate}
                placeholder="outcome >= warning"
                value={form.filter}
                onChange={event => setForm(current => ({ ...current, filter: event.target.value }))}
              />
            </div>
            <div className="flex items-center gap-3">
              <label className="flex items-center gap-2 text-xs text-muted">
                <Switch
                  aria-label="Create notification preset enabled"
                  checked={form.enabled}
                  disabled={!canMutate || createPending}
                  onCheckedChange={next => setForm(current => ({ ...current, enabled: next }))}
                  data-testid="settings-page-hooks-notification-preset-enabled"
                />
                enabled
              </label>
              <div className="ml-auto flex items-center gap-2">
                <Button
                  data-testid="settings-page-hooks-notification-preset-cancel"
                  onClick={() => {
                    setCreateOpen(false);
                    setLocalError(null);
                  }}
                  disabled={createPending}
                  size="sm"
                  type="button"
                  variant="ghost"
                >
                  Cancel
                </Button>
                <Button
                  data-testid="settings-page-hooks-notification-preset-create"
                  disabled={!canMutate || createPending}
                  onClick={submit}
                  type="button"
                  size="sm"
                >
                  {createPending ? <Spinner className="size-3.5" /> : <Plus className="size-3.5" />}
                  Create
                </Button>
              </div>
            </div>
          </div>
        ) : null}

        {localError ? (
          <span
            className="text-xs text-danger"
            data-testid="settings-page-hooks-notification-presets-error"
            id={formErrorId}
            role="alert"
          >
            {localError}
          </span>
        ) : null}

        {error ? (
          <span
            className="text-xs text-danger"
            data-testid="settings-page-hooks-notification-presets-operational-error"
            role="alert"
          >
            {error}
          </span>
        ) : null}

        {isLoading && presets.length === 0 ? (
          <SkeletonRows
            aria-label="Loading notification presets"
            className="gap-2"
            count={3}
            data-testid="settings-page-hooks-notification-presets-loading"
            role="status"
            rowClassName="rounded-md border border-line px-3 py-3"
          />
        ) : presets.length === 0 ? (
          <Empty
            icon={Bell}
            title="No notification presets"
            description="SQLite has no preset rows for this runtime."
            data-testid="settings-page-hooks-notification-presets-empty"
          />
        ) : (
          <ul
            className="overflow-hidden rounded-lg border border-line"
            data-testid="settings-page-hooks-notification-presets-list"
          >
            {presets.map(preset => (
              <NotificationPresetRow
                key={preset.name}
                preset={preset}
                mutationPending={pendingName !== null}
                showPending={pendingName === preset.name}
                canMutate={canMutate}
                onToggle={onToggle}
                onDelete={onDelete}
              />
            ))}
          </ul>
        )}
      </div>
    </Section>
  );
}

function NotificationPresetRow({
  preset,
  mutationPending,
  showPending,
  canMutate,
  onToggle,
  onDelete,
}: {
  preset: NotificationPresetEntry;
  mutationPending: boolean;
  showPending: boolean;
  canMutate: boolean;
  onToggle: (preset: NotificationPresetEntry, nextEnabled: boolean) => void;
  onDelete: (preset: NotificationPresetEntry) => void;
}) {
  const deleteDisabled = !canMutate || mutationPending || preset.built_in;
  const deleteButton = (
    <Button
      aria-label={"Delete " + preset.name}
      data-testid={"settings-page-hooks-notification-preset-row-" + preset.name + "-delete"}
      disabled={deleteDisabled}
      onClick={() => onDelete(preset)}
      size="icon-sm"
      type="button"
      variant="ghost"
    >
      <Trash2 className="size-3.5" />
    </Button>
  );

  return (
    <li
      className="grid grid-cols-[26px_minmax(0,1fr)_auto] items-center gap-3 border-b border-line-soft bg-canvas-soft px-4 py-2.5 last:border-b-0"
      data-testid={"settings-page-hooks-notification-preset-row-" + preset.name}
    >
      <span
        aria-hidden="true"
        className="flex size-6.5 items-center justify-center rounded-sm bg-elevated text-subtle"
      >
        <Bell className="size-3.5" />
      </span>
      <div className="flex min-w-0 flex-col gap-1">
        <span className="flex min-w-0 flex-wrap items-center gap-1.5">
          <span className="truncate font-mono text-sm text-fg">{preset.name}</span>
          <Pill mono size="xs">
            {preset.built_in ? "built-in" : "custom"}
          </Pill>
          {preset.user_modified ? (
            <Pill mono size="xs" tone="warning">
              modified
            </Pill>
          ) : null}
          {preset.default_update_available ? (
            <Pill mono size="xs" tone="warning">
              default drift
            </Pill>
          ) : null}
        </span>
        <span className="truncate font-mono text-mono-id text-subtle">
          {preset.events.join(", ")}
          <span aria-hidden="true" className="px-1 text-faint">
            →
          </span>
          {notificationPresetTargetLabel(preset.targets)}
        </span>
      </div>
      <div className="flex items-center justify-end gap-2">
        {showPending ? <Spinner className="size-3.5 text-muted" /> : null}
        <Switch
          aria-label={"Toggle " + preset.name}
          checked={preset.enabled}
          disabled={!canMutate || mutationPending}
          onCheckedChange={next => onToggle(preset, next)}
          data-testid={"settings-page-hooks-notification-preset-row-" + preset.name + "-toggle"}
        />
        {preset.built_in ? (
          <Tooltip>
            <TooltipTrigger render={<span className="inline-flex" />}>
              {deleteButton}
            </TooltipTrigger>
            <TooltipContent>Built-in presets cannot be deleted.</TooltipContent>
          </Tooltip>
        ) : (
          deleteButton
        )}
      </div>
    </li>
  );
}

function parseNotificationPresetTargetEntry(
  value: string
): CreateNotificationPresetRequest["targets"] | null {
  const trimmed = value.trim();
  if (trimmed === "") return [];
  const split = trimmed.indexOf(":");
  if (split <= 0 || split === trimmed.length - 1) return null;
  return [
    {
      bridge_id: trimmed.slice(0, split).trim(),
      canonical_route: trimmed.slice(split + 1).trim(),
      delivery_mode: "direct-send",
    },
  ];
}

function notificationPresetTargetLabel(targets: NotificationPresetEntry["targets"]) {
  if (targets.length === 0) return "none";
  return targets
    .map(target => {
      const route = target.canonical_route ?? target.display_name ?? "unresolved";
      return target.bridge_id + ":" + route;
    })
    .join(", ");
}
