import { AlertCircle } from "lucide-react";
import { useState, type Dispatch, type SetStateAction } from "react";

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
  useSettingsProviders,
  useSettingsSandboxes,
  type SettingsGeneralSection,
} from "@/systems/settings";
import { ToolApprovalGrantsSection } from "@/systems/tool-approvals";
import {
  Button,
  Input,
  NativeSelect,
  NativeSelectOption,
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  Spinner,
} from "@agh/ui";

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

type GeneralConfig = SettingsGeneralSection["config"];

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
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-general-loading"
      >
        <Spinner className="size-5 text-subtle" />
      </div>
    );
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-general-error"
      >
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle className="size-6 text-danger" />
          <p className="text-sm text-subtle">
            {page.error?.message ?? "Failed to load general settings"}
          </p>
          <Button onClick={page.handleRetry} size="sm" type="button" variant="outline">
            Retry
          </Button>
        </div>
      </div>
    );
  }

  const { envelope, draft, setDraft, restart, update } = page;
  const runtime = envelope.runtime;

  return (
    <SettingsPageFrame
      description="Defaults and day-to-day behavior for this runtime. Changes here apply to new sessions on this machine."
      meta={
        runtime.available
          ? [
              {
                key: "sessions",
                content: (
                  <span>
                    <span className="font-medium text-muted">{runtime.active_sessions}</span> active
                    sessions
                  </span>
                ),
              },
              {
                key: "agents",
                content: (
                  <span>
                    <span className="font-medium text-muted">{runtime.active_agents}</span> agents
                    working
                  </span>
                ),
              },
            ]
          : [{ key: "runtime", content: <span>runtime unavailable</span> }]
      }
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
      <DefaultsGroup draft={draft} setDraft={setDraft} />

      <SettingsGroup
        data-testid="settings-page-general-permissions"
        description="How much agents can do before asking you first."
        title="Permissions"
      >
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

      <SettingsGroup description="Lifecycle for sessions this daemon hosts." title="Sessions">
        <SettingRow
          data-testid="settings-page-general-session-timeout"
          description="A session with no activity for this long is ended and kept in history. 0 keeps sessions open."
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

      <SettingsGroup
        bare
        description="Where this daemon is listening right now. Read-only."
        title="Runtime"
      >
        {runtime.available ? (
          <SettingsTiles>
            <SettingsTile
              label="Local socket"
              mono
              value={runtime.socket ?? envelope.config.daemon.socket}
            />
            <SettingsTile
              label="HTTP address"
              mono
              value={
                runtime.http_host && runtime.http_port
                  ? `${runtime.http_host}:${runtime.http_port}`
                  : `${envelope.config.http.host}:${envelope.config.http.port}`
              }
            />
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
        ) : (
          <SettingsRuntimeUnavailable
            slug="general"
            description="Session, agent, socket, and uptime facts could not be measured."
          />
        )}
      </SettingsGroup>

      <GeneralUpdateSection
        data={update.data}
        error={update.error}
        isError={update.isError}
        isFetching={update.isFetching}
        isLoading={update.isLoading}
        onRetry={() => void update.refetch()}
      />

      <SettingsAdvancedFold data-testid="settings-page-general-advanced">
        <SettingRow
          data-testid="settings-page-general-reload"
          description="Re-read the config file without restarting the daemon."
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
          description={
            page.applyRecords.data?.entries?.length
              ? `${page.applyRecords.data.entries.length} apply records.`
              : "History of applied configuration changes."
          }
          label="Configuration changes"
          onClick={() => setApplyRecordsOpen(true)}
        />
        <SettingRow
          description="Workspace overlays can override values from this file."
          label="Config file"
          control={<SettingValue mono>{envelope.config_paths?.global_config ?? "—"}</SettingValue>}
        />
        {update.data ? (
          <SettingRow
            data-testid="settings-page-general-update-detail"
            description={
              update.data.recommendation ??
              "Latest stable and install-method detail for this machine."
            }
            label="Update detail"
            control={
              <SettingValue mono>
                {update.data.latest_version ?? "—"} · {update.data.install_method ?? "—"}
              </SettingValue>
            }
          />
        ) : null}
      </SettingsAdvancedFold>

      <Sheet onOpenChange={setApplyRecordsOpen} open={applyRecordsOpen}>
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
              error={page.applyRecords.error instanceof Error ? page.applyRecords.error : null}
              reloadError={page.reloadError}
              reloadResult={page.reloadResult}
              isReloading={page.isReloading}
              onRefresh={() => void page.applyRecords.refetch()}
              onReload={page.handleReload}
            />
          </div>
        </SheetContent>
      </Sheet>
    </SettingsPageFrame>
  );
}

interface DraftSectionProps {
  draft: GeneralConfig;
  setDraft: Dispatch<SetStateAction<GeneralConfig | null>>;
}

function DefaultsGroup({ draft, setDraft }: DraftSectionProps) {
  const providers = useSettingsProviders();
  const sandboxes = useSettingsSandboxes();
  const providerNames = (providers.data?.providers ?? []).map(entry => entry.name);
  const sandboxNames = (sandboxes.data?.sandboxes ?? []).map(entry => entry.name);

  return (
    <SettingsGroup
      data-testid="settings-page-general-defaults"
      description="What new sessions start with unless you pick something else."
      title="Defaults"
    >
      <SettingRow
        data-testid="settings-page-general-default-agent"
        description="Used when you start a session without choosing an agent."
        label="Default agent"
        control={
          <Input
            className="w-52"
            data-testid="settings-page-general-default-agent-input"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return { ...current, defaults: { ...current.defaults, agent: event.target.value } };
              })
            }
            value={draft.defaults.agent}
          />
        }
      />
      <SettingRow
        data-testid="settings-page-general-default-provider"
        description="The provider new agents run on when none is set."
        label="Default provider"
        control={
          <NativeSelect
            className="w-52"
            data-testid="settings-page-general-default-provider-input"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  defaults: { ...current.defaults, provider: event.target.value },
                };
              })
            }
            value={draft.defaults.provider ?? ""}
          >
            <NativeSelectOption value="">auto</NativeSelectOption>
            {providerNames.map(name => (
              <NativeSelectOption key={name} value={name}>
                {name}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        }
      />
      <SettingRow
        data-testid="settings-page-general-default-sandbox"
        description="How much of your file system new sessions can touch."
        label="Default sandbox"
        control={
          <NativeSelect
            className="w-52 font-mono"
            data-testid="settings-page-general-default-sandbox-input"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  defaults: { ...current.defaults, sandbox: event.target.value },
                };
              })
            }
            value={draft.defaults.sandbox ?? ""}
          >
            <NativeSelectOption value="">local</NativeSelectOption>
            {sandboxNames.map(name => (
              <NativeSelectOption key={name} value={name}>
                {name}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        }
      />
    </SettingsGroup>
  );
}
