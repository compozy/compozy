import { AlertCircle } from "lucide-react";
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
  SettingsNumberInput,
  SettingsPageFrame,
  SettingsProvChip,
  SettingsSaveBar,
  SettingsRuntimeUnavailable,
  useSettingsSaveBarState,
  useSettingsTopbar,
  SettingsTile,
  SettingsTiles,
} from "@/systems/settings";
import { DEFAULT_SESSION_BUSY_INPUT_MODE, type SessionBusyInputMode } from "@/systems/session";
import { ToolApprovalGrantsSection } from "@/systems/tool-approvals";
import {
  Button,
  PillGroup,
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  Spinner,
  type PillGroupItem,
} from "@compozy/ui";

import { DaemonSection, RedactionSection } from "./-general-daemon-sections";
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

type GeneralPageModel = ReturnType<typeof useSettingsGeneralPage>;
type GeneralEnvelope = NonNullable<GeneralPageModel["envelope"]>;
type GeneralRuntime = GeneralEnvelope["runtime"];

function GeneralSettingsLoading() {
  return (
    <div
      className="flex flex-1 items-center justify-center"
      data-testid="settings-page-general-loading"
    >
      <Spinner className="size-5 text-subtle" />
    </div>
  );
}

function GeneralSettingsLoadError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div
      className="flex flex-1 items-center justify-center"
      data-testid="settings-page-general-error"
    >
      <div className="flex flex-col items-center gap-2 text-center">
        <AlertCircle className="size-6 text-danger" />
        <p className="text-sm text-subtle">{message}</p>
        <Button onClick={onRetry} size="sm" type="button" variant="outline">
          Retry
        </Button>
      </div>
    </div>
  );
}

/** The frame's meta strip: live session/agent counts, or the one fact that the runtime is unreachable. */
function generalSettingsMeta(runtime: GeneralRuntime) {
  if (!runtime.available) {
    return [{ key: "runtime", content: <span>runtime unavailable</span> }];
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

/** Read-only runtime facts, or the shared unavailable notice when they could not be measured. */
function GeneralRuntimeSection({ envelope }: { envelope: GeneralEnvelope }) {
  const runtime = envelope.runtime;
  if (!runtime.available) {
    return (
      <SettingsGroup bare description="Read-only." title="Runtime">
        <SettingsRuntimeUnavailable
          slug="general"
          description="Session, agent, socket, and uptime facts could not be measured."
        />
      </SettingsGroup>
    );
  }
  const httpAddress =
    runtime.http_host && runtime.http_port
      ? `${runtime.http_host}:${runtime.http_port}`
      : `${envelope.config.http.host}:${envelope.config.http.port}`;
  return (
    <SettingsGroup bare description="Read-only." title="Runtime">
      <SettingsTiles>
        <SettingsTile
          label="Local socket"
          mono
          value={runtime.socket ?? envelope.config.daemon.socket}
        />
        <SettingsTile label="HTTP address" mono value={httpAddress} />
        <SettingsTile
          dotTone={runtime.active_sessions > 0 ? "success" : "neutral"}
          label="Active sessions"
          value={String(runtime.active_sessions)}
        />
        <SettingsTile
          label="Agents running"
          value={`${runtime.active_agents} of ${envelope.config.limits.max_concurrent_agents} max`}
        />
      </SettingsTiles>
    </SettingsGroup>
  );
}

/** Reload, apply-record history, config-file provenance, and the update detail fold. */
function GeneralAdvancedSection({
  envelope,
  onOpenApplyRecords,
  page,
}: {
  envelope: GeneralEnvelope;
  onOpenApplyRecords: () => void;
  page: GeneralPageModel;
}) {
  const applyRecordCount = page.applyRecords.data?.entries?.length ?? 0;
  const updateRuntime = page.update.data?.runtime;
  return (
    <SettingsAdvancedFold data-testid="settings-page-general-advanced">
      <SettingRow
        data-testid="settings-page-general-reload"
        description="Re-read the config file without restarting CompozyOS."
        label={
          <>
            Reload configuration <SettingsProvChip>config.toml</SettingsProvChip>
          </>
        }
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
        description={applyRecordCount > 0 ? `${applyRecordCount} apply records.` : undefined}
        label="Configuration changes"
        onClick={onOpenApplyRecords}
      />
      <SettingRow
        description="Workspace overlays can override values from this file."
        label="Config file"
        control={<SettingValue mono>{envelope.config_paths?.global_config ?? "—"}</SettingValue>}
      />
      {updateRuntime ? (
        <SettingRow
          data-testid="settings-page-general-update-detail"
          description={updateRuntime.recommendation ?? undefined}
          label="Update detail"
          control={
            <SettingValue mono>
              {updateRuntime.latest_version ?? "—"} · {updateRuntime.install_method || "—"}
            </SettingValue>
          }
        />
      ) : null}
    </SettingsAdvancedFold>
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
          <SheetTitle>Configuration changes</SheetTitle>
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
  const [validationErrors, setValidationErrors] = useState<Record<string, string | null>>({});
  const [applyRecordsOpen, setApplyRecordsOpen] = useState(false);
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

  if (page.isLoading) {
    return <GeneralSettingsLoading />;
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <GeneralSettingsLoadError
        message={page.error?.message ?? "Failed to load general settings"}
        onRetry={page.handleRetry}
      />
    );
  }

  const { envelope, draft, setDraft, restart, update } = page;

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
          description="0 keeps sessions open."
          error={validationErrors.sessionTimeout ?? undefined}
          label="End idle sessions after"
          control={
            <span className="flex items-center gap-2">
              <SettingsNumberInput
                className="w-28 text-right font-mono"
                data-testid="settings-page-general-session-timeout-input"
                min={0}
                onValidityChange={setValidationError("sessionTimeout")}
                onValueChange={value =>
                  setDraft(prev => {
                    const current = prev ?? draft;
                    return { ...current, session_timeout: formatSessionTimeout(value) };
                  })
                }
                value={parseSessionTimeoutSeconds(draft.session_timeout)}
              />
              <span className="text-form-label text-subtle">seconds</span>
            </span>
          }
        />
      </SettingsGroup>

      <DaemonSection draft={draft} setDraft={setDraft} />
      <RedactionSection draft={draft} setDraft={setDraft} />

      <GeneralRuntimeSection envelope={envelope} />

      <GeneralUpdateSection
        actions={page.updateActions}
        data={update.data}
        error={update.error}
        isError={update.isError}
        isFetching={update.isFetching}
        isLoading={update.isLoading}
        onRetry={() => void update.refetch()}
      />

      <GeneralAdvancedSection
        envelope={envelope}
        onOpenApplyRecords={() => setApplyRecordsOpen(true)}
        page={page}
      />

      <GeneralApplyRecordsSheet
        onOpenChange={setApplyRecordsOpen}
        open={applyRecordsOpen}
        page={page}
      />
    </SettingsPageFrame>
  );
}
