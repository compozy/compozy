import { useState } from "react";

import { useSettingsGeneralPage } from "@/systems/settings/hooks/use-settings-general-page";
import {
  SettingActionRow,
  SettingRow,
  SettingValue,
  SettingsAdvancedFold,
  SettingsApplyRecordsPanel,
  SettingsChoiceGroup,
  SettingsGroup,
  SettingsPageFrame,
  SettingsPageState,
  SettingsSaveBar,
  useSettingsSaveBarState,
  useSettingsTopbar,
} from "@/systems/settings";
import { DEFAULT_SESSION_BUSY_INPUT_MODE, type SessionBusyInputMode } from "@/systems/session";
import { ToolApprovalGrantsSection } from "@/systems/tool-approvals";
import {
  Button,
  NativeSelect,
  NativeSelectOption,
  PillGroup,
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  Spinner,
  type PillGroupItem,
} from "@compozy/ui";

import { MemoryReportRow, RedactionSection } from "./-general-daemon-sections";
import { generalUpdateNeedsAttention } from "./-general-update-attention";
import { GeneralUpdateSection } from "./-general-update-section";

const PERMISSION_OPTIONS = [
  {
    value: "deny-all" as const,
    name: "Ask first",
    description: "Every action waits for your approval.",
  },
  {
    value: "approve-reads" as const,
    name: "Allow reading",
    description: "Reads run on their own; changes still ask.",
  },
  {
    value: "approve-all" as const,
    name: "Allow everything",
    description: "Agents act freely. For trusted work only.",
  },
];

/**
 * Follow-up behavior mirrors `session.busy_input.default_mode` — daemon-owned,
 * so the composer, the CLI, and native tools resolve the same default (ADR-002).
 */
const FOLLOW_UP_OPTIONS: ReadonlyArray<PillGroupItem<SessionBusyInputMode>> = [
  {
    value: "steer",
    label: "Steer immediately",
    testId: "settings-page-general-follow-up-steer",
  },
  {
    value: "queue",
    label: "Queue until the turn ends",
    testId: "settings-page-general-follow-up-queue",
  },
];

function followUpModeFromConfig(config: {
  busy_input?: { default_mode: string } | null;
}): SessionBusyInputMode {
  return config.busy_input?.default_mode === "queue" ? "queue" : DEFAULT_SESSION_BUSY_INPUT_MODE;
}

function parseSessionTimeoutSeconds(raw: string): number {
  if (!raw) return 0;
  const match = /^(\d+)(s|m|h)?$/i.exec(raw.trim());
  if (!match) return 0;
  const value = Number.parseInt(match[1] ?? "0", 10);
  const unit = (match[2] ?? "s").toLowerCase();
  if (unit === "h") return value * 3600;
  if (unit === "m") return value * 60;
  return value;
}

function formatSessionTimeout(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "0s";
  return `${Math.floor(seconds)}s`;
}

const SESSION_TIMEOUT_PRESETS: ReadonlyArray<{ seconds: number; label: string }> = [
  { seconds: 0, label: "Never" },
  { seconds: 15 * 60, label: "15 minutes" },
  { seconds: 60 * 60, label: "1 hour" },
  { seconds: 4 * 60 * 60, label: "4 hours" },
  { seconds: 24 * 60 * 60, label: "1 day" },
];

const CUSTOM_TIMEOUT = "custom";

/** Duration in plain words for a value that matches no preset ("90 minutes"). */
function describeSeconds(seconds: number): string {
  if (seconds % 3600 === 0) return `${seconds / 3600} hours`;
  if (seconds % 60 === 0) return `${seconds / 60} minutes`;
  return `${seconds} seconds`;
}

type GeneralPageModel = ReturnType<typeof useSettingsGeneralPage>;
type GeneralEnvelope = NonNullable<GeneralPageModel["envelope"]>;
type GeneralRuntime = GeneralEnvelope["runtime"];

/** The frame's meta strip: live session/agent counts, or the one fact that the runtime is unreachable. */
function generalSettingsMeta(runtime: GeneralRuntime) {
  if (!runtime.available) {
    return [{ key: "runtime", content: <span>CompozyOS isn&apos;t reachable</span> }];
  }
  return [
    {
      key: "sessions",
      content: (
        <span>
          <span className="font-medium text-muted">{runtime.active_sessions}</span> active sessions
        </span>
      ),
    },
    {
      key: "agents",
      content: (
        <span>
          <span className="font-medium text-muted">{runtime.active_agents}</span> agents working
        </span>
      ),
    },
  ];
}

/** Where CompozyOS listens — the local socket and HTTP address, from live runtime facts when available. */
function listenAddresses(envelope: GeneralEnvelope) {
  const runtime = envelope.runtime;
  const live = runtime.available ? runtime : null;
  return {
    socket: live?.socket ?? envelope.config.daemon.socket,
    http:
      live?.http_host && live.http_port
        ? `${live.http_host}:${live.http_port}`
        : `${envelope.config.http.host}:${envelope.config.http.port}`,
  };
}

/**
 * The operator layer: settings-file reload and history, where CompozyOS
 * listens, memory-usage logging, and update internals.
 */
function GeneralAdvancedSection({
  draft,
  envelope,
  onOpenApplyRecords,
  page,
  setDraft,
}: {
  draft: GeneralEnvelope["config"];
  envelope: GeneralEnvelope;
  onOpenApplyRecords: () => void;
  page: GeneralPageModel;
  setDraft: GeneralPageModel["setDraft"];
}) {
  const applyRecordCount = page.applyRecords.data?.entries?.length ?? 0;
  const updateRuntime = page.update.data?.runtime;
  const addresses = listenAddresses(envelope);
  return (
    <SettingsAdvancedFold data-testid="settings-page-general-advanced">
      <SettingRow
        data-testid="settings-page-general-reload"
        description="Re-read the settings file without restarting CompozyOS."
        label="Reload settings file"
        control={
          <Button
            data-testid="settings-page-general-reload-button"
            disabled={page.isReloading}
            onClick={page.handleReload}
            size="sm"
            type="button"
            variant="neutral"
          >
            {page.isReloading ? <Spinner className="size-3" /> : null}
            Reload
          </Button>
        }
      />
      <SettingActionRow
        data-testid="settings-page-general-apply-records"
        description={
          applyRecordCount > 0
            ? `${applyRecordCount} saved ${applyRecordCount === 1 ? "change" : "changes"}.`
            : undefined
        }
        label="Settings file changes"
        onClick={onOpenApplyRecords}
      />
      <SettingRow
        description="Project settings can override this file."
        label="Settings file"
        control={<SettingValue mono>{envelope.config_paths?.global_config ?? "—"}</SettingValue>}
      />
      <SettingRow
        data-testid="settings-page-general-socket"
        label="Local socket"
        control={<SettingValue mono>{addresses.socket}</SettingValue>}
      />
      <SettingRow
        data-testid="settings-page-general-http-address"
        label="Local address"
        control={<SettingValue mono>{addresses.http}</SettingValue>}
      />
      <MemoryReportRow draft={draft} setDraft={setDraft} />
      {updateRuntime ? (
        <SettingRow
          data-testid="settings-page-general-update-detail"
          label="Update detail"
          control={
            <SettingValue mono>
              {updateRuntime.latest_version ?? "—"} · {updateRuntime.install_method || "—"}
            </SettingValue>
          }
        />
      ) : null}
      {updateRuntime?.recommendation ? (
        <SettingRow
          data-testid="settings-page-general-update-recommendation"
          label="Upgrade command"
          control={<SettingValue mono>{updateRuntime.recommendation}</SettingValue>}
        />
      ) : null}
    </SettingsAdvancedFold>
  );
}

/** Idle-session cutoff as presets; a value that matches none stays selectable as-is. */
function SessionTimeoutSelect({
  disabled,
  onChange,
  value,
}: {
  disabled: boolean;
  onChange: (seconds: number) => void;
  value: string;
}) {
  const seconds = parseSessionTimeoutSeconds(value);
  const preset = SESSION_TIMEOUT_PRESETS.find(option => option.seconds === seconds);
  return (
    <NativeSelect
      aria-label="End idle sessions after"
      data-testid="settings-page-general-session-timeout-input"
      disabled={disabled}
      onChange={event => {
        const next = event.target.value;
        if (next !== CUSTOM_TIMEOUT) onChange(Number(next));
      }}
      size="sm"
      value={preset ? String(preset.seconds) : CUSTOM_TIMEOUT}
    >
      {SESSION_TIMEOUT_PRESETS.map(option => (
        <NativeSelectOption key={option.seconds} value={String(option.seconds)}>
          {option.label}
        </NativeSelectOption>
      ))}
      {preset ? null : (
        <NativeSelectOption value={CUSTOM_TIMEOUT}>{describeSeconds(seconds)}</NativeSelectOption>
      )}
    </NativeSelect>
  );
}

function GeneralApplyRecordsSheet({
  onOpenChange,
  open,
  page,
}: {
  onOpenChange: (open: boolean) => void;
  open: boolean;
  page: GeneralPageModel;
}) {
  const error = page.applyRecords.error;
  return (
    <Sheet onOpenChange={onOpenChange} open={open}>
      <SheetContent
        className="w-[min(var(--width-settings-sheet),calc(100vw-var(--spacing-settings-sheet-viewport-gutter)))] sm:max-w-none"
        data-testid="settings-page-general-apply-records-sheet"
        side="right"
      >
        <SheetHeader>
          <SheetTitle>Settings file changes</SheetTitle>
        </SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
          <SettingsApplyRecordsPanel
            records={page.applyRecords.data?.entries ?? []}
            isLoading={page.applyRecords.isLoading}
            isFetching={page.applyRecords.isFetching}
            error={error instanceof Error ? error : null}
            reloadError={page.reloadError}
            reloadResult={page.reloadResult}
            isReloading={page.isReloading}
            onRefresh={() => void page.applyRecords.refetch()}
            onReload={page.handleReload}
          />
        </div>
      </SheetContent>
    </Sheet>
  );
}

export function GeneralSettingsPage() {
  const page = useSettingsGeneralPage();
  useSettingsTopbar("general");
  const [applyRecordsOpen, setApplyRecordsOpen] = useState(false);
  const saveBarState = useSettingsSaveBarState({
    isDirty: page.isDirty,
    isInvalid: false,
    isSaving: page.isSaving,
    error: page.saveError,
    warnings: page.warnings,
    lastAppliedLabel: page.lastAppliedLabel,
  });

  if (page.isLoading) {
    return <SettingsPageState slug="general" state="loading" />;
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <SettingsPageState
        error={page.error}
        onRetry={page.handleRetry}
        slug="general"
        state="error"
      />
    );
  }

  const { envelope, draft, setDraft, restart, update } = page;
  const updateSection = (
    <GeneralUpdateSection
      actions={page.updateActions}
      data={update.data}
      error={update.error}
      isError={update.isError}
      isFetching={update.isFetching}
      isLoading={update.isLoading}
      onRetry={() => void update.refetch()}
    />
  );
  // Updates lead the page only when there is something to act on.
  const updatesFirst = generalUpdateNeedsAttention(update);

  return (
    <SettingsPageFrame
      description="Changes here apply to new sessions on this machine."
      meta={generalSettingsMeta(envelope.runtime)}
      restart={restart}
      saveBar={
        <SettingsSaveBar
          slug="general"
          state={saveBarState}
          onSave={page.handleSave}
          onReset={page.handleReset}
        />
      }
      slug="general"
    >
      {updatesFirst ? updateSection : null}

      <SettingsGroup data-testid="settings-page-general-permissions" title="Permissions">
        <SettingsChoiceGroup
          ariaLabel="Permission mode"
          data-testid="settings-page-general-permissions-group"
          onChange={mode =>
            setDraft(prev => {
              const current = prev ?? draft;
              return { ...current, permissions: { mode } };
            })
          }
          options={PERMISSION_OPTIONS}
          value={draft.permissions.mode}
        />
      </SettingsGroup>

      <ToolApprovalGrantsSection />

      <SettingsGroup title="Sessions">
        <SettingRow
          data-testid="settings-page-general-follow-up"
          label="Follow-up behavior"
          control={
            <PillGroup
              aria-label="Follow-up behavior"
              data-testid="settings-page-general-follow-up-group"
              items={FOLLOW_UP_OPTIONS.map(item => ({ ...item, disabled: page.isSaving }))}
              onChange={mode =>
                setDraft(prev => {
                  const current = prev ?? draft;
                  return { ...current, busy_input: { default_mode: mode } };
                })
              }
              size="sm"
              value={followUpModeFromConfig(draft)}
            />
          }
        />
        <SettingRow
          data-testid="settings-page-general-session-timeout"
          help="A session with no activity for this long is ended and kept in history."
          label="End idle sessions after"
          control={
            <SessionTimeoutSelect
              disabled={page.isSaving}
              onChange={value =>
                setDraft(prev => {
                  const current = prev ?? draft;
                  return { ...current, session_timeout: formatSessionTimeout(value) };
                })
              }
              value={draft.session_timeout}
            />
          }
        />
      </SettingsGroup>

      <RedactionSection draft={draft} setDraft={setDraft} />

      {updatesFirst ? null : updateSection}

      <GeneralAdvancedSection
        draft={draft}
        envelope={envelope}
        onOpenApplyRecords={() => setApplyRecordsOpen(true)}
        page={page}
        setDraft={setDraft}
      />

      <GeneralApplyRecordsSheet
        onOpenChange={setApplyRecordsOpen}
        open={applyRecordsOpen}
        page={page}
      />
    </SettingsPageFrame>
  );
}
