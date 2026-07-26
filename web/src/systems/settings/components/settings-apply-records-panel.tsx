import { AlertCircle, CheckCircle2, RefreshCw, RotateCw } from "lucide-react";

import type { ConfigApplyRecord, SettingsApplyResponse } from "@/systems/settings/types";
import { Alert, AlertDescription, Button, Empty, Pill, Section, Skeleton, Spinner } from "@agh/ui";
import type { PillTone } from "@agh/ui";

interface SettingsApplyRecordsPanelProps {
  records: ConfigApplyRecord[];
  isLoading: boolean;
  isFetching?: boolean;
  error?: Error | null;
  reloadError?: string | null;
  reloadResult?: SettingsApplyResponse | null;
  isReloading: boolean;
  onRefresh: () => void;
  onReload: () => void;
}

const STATUS_TONE: Record<ConfigApplyRecord["status"], PillTone> = {
  pending_apply: "info",
  applied: "success",
  blocked: "warning",
  failed: "danger",
};

const NEXT_ACTION_LABEL: Record<ConfigApplyRecord["next_action"], string> = {
  none: "none",
  "restart-daemon": "restart daemon",
  "new-session": "new session",
  retry: "retry",
};

function formatDateTime(value?: string | null): string {
  if (!value) return "--";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? "--" : parsed.toLocaleString();
}

function shortHash(value: string): string {
  return value.length <= 12 ? value : value.slice(0, 12);
}

function normalizeLabel(value: string): string {
  return value.replace(/-/g, " ").replace(/_/g, " ");
}

function reloadResultMessage(result: SettingsApplyResponse): string {
  if (result.skipped) {
    return result.skipped_reason ?? "No config changes detected";
  }
  if (result.next_action === "restart-daemon") {
    return `Apply blocked at generation ${result.active_generation}. Restart the daemon to make the desired config active.`;
  }
  if (result.next_action === "new-session") {
    return `Active generation ${result.active_generation} is ready for new sessions.`;
  }
  if (result.next_action === "retry") {
    return "Reload failed. Fix the diagnostics and retry.";
  }
  return `Active generation ${result.active_generation} applied.`;
}

function latestDiagnostic(record: ConfigApplyRecord): string | null {
  const diagnostic = record.diagnostics?.[0];
  if (!diagnostic) return null;
  return `${diagnostic.title}: ${diagnostic.message}`;
}

function SettingsApplyRecordsPanel({
  records,
  isLoading,
  isFetching = false,
  error,
  reloadError,
  reloadResult,
  isReloading,
  onRefresh,
  onReload,
}: SettingsApplyRecordsPanelProps) {
  return (
    <Section
      divided
      label="Config apply history"
      note="config.toml is desired state. Active generation is runtime truth."
      count={records.length}
      right={
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={onRefresh}
            disabled={isFetching}
            data-testid="settings-apply-records-refresh"
          >
            {isFetching ? <Spinner className="size-3" /> : <RefreshCw className="size-3" />}
            Refresh
          </Button>
          <Button
            type="button"
            variant="default"
            size="sm"
            onClick={onReload}
            disabled={isReloading}
            data-testid="settings-apply-records-reload"
          >
            {isReloading ? <Spinner className="size-3" /> : <RotateCw className="size-3" />}
            Reload config
          </Button>
        </div>
      }
      data-testid="settings-apply-records-panel"
    >
      <div className="flex flex-col gap-3">
        {reloadError ? (
          <Alert variant="danger" data-testid="settings-apply-records-reload-error">
            <AlertCircle className="mt-0.5 size-3 shrink-0" />
            <AlertDescription className="text-xs">{reloadError}</AlertDescription>
          </Alert>
        ) : reloadResult ? (
          <Alert
            variant={reloadResult.next_action === "restart-daemon" ? "warning" : "success"}
            data-testid="settings-apply-records-reload-result"
          >
            <CheckCircle2 className="mt-0.5 size-3 shrink-0" />
            <AlertDescription className="text-xs">
              {reloadResultMessage(reloadResult)}
            </AlertDescription>
          </Alert>
        ) : null}

        {error ? (
          <Alert variant="danger" data-testid="settings-apply-records-error">
            <AlertCircle className="mt-0.5 size-3 shrink-0" />
            <AlertDescription className="text-xs">{error.message}</AlertDescription>
          </Alert>
        ) : null}

        {isLoading ? (
          <div
            aria-label="Loading apply records"
            className="flex flex-col divide-y divide-line border-y border-line"
            role="status"
          >
            {[0, 1, 2].map(row => (
              <div
                className="grid min-w-0 gap-4 py-3 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)_minmax(0,1.4fr)]"
                key={row}
              >
                <Skeleton className="h-8 w-full rounded-xs" />
                <Skeleton className="h-8 w-full rounded-xs" />
                <Skeleton className="h-8 w-full rounded-xs" />
              </div>
            ))}
          </div>
        ) : records.length === 0 ? (
          <Empty
            title="No apply records"
            description="Config reload and settings mutations will appear here."
            data-testid="settings-apply-records-empty"
          />
        ) : (
          <div
            className="divide-y divide-line border-y border-line"
            data-testid="settings-apply-records-list"
          >
            {records.map(record => (
              <div
                key={record.id}
                className="grid min-w-0 gap-4 py-3 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)_minmax(0,1.4fr)]"
                data-testid="settings-apply-records-row"
              >
                <div className="flex min-w-0 flex-col gap-2">
                  <span className="eyebrow text-muted">Status</span>
                  <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                    <Pill tone={STATUS_TONE[record.status]} size="xs">
                      <Pill.Dot />
                      {normalizeLabel(record.status)}
                    </Pill>
                    <Pill mono tone="neutral" size="xs">
                      {record.lifecycle}
                    </Pill>
                  </div>
                  <div className="flex min-w-0 flex-wrap gap-x-4 gap-y-1 text-small-body text-muted">
                    <span>
                      generation{" "}
                      <span className="font-mono text-mono-id font-medium text-fg tabular-nums">
                        {record.generation}
                      </span>
                    </span>
                    <span>actor {record.actor}</span>
                  </div>
                </div>
                <div className="flex min-w-0 flex-col gap-2">
                  <span className="eyebrow text-muted">Hashes</span>
                  <div className="grid min-w-0 gap-1 font-mono text-mono-id text-muted">
                    <span className="min-w-0 truncate">
                      desired {shortHash(record.desired_config_hash)}
                    </span>
                    <span className="min-w-0 truncate">
                      active {shortHash(record.active_config_hash)}
                    </span>
                  </div>
                </div>
                <div className="flex min-w-0 flex-col gap-2">
                  <div className="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-1 text-small-body text-muted">
                    <span>updated {formatDateTime(record.updated_at)}</span>
                    <span>next {NEXT_ACTION_LABEL[record.next_action]}</span>
                  </div>
                  <p className="text-small-body leading-relaxed text-muted">
                    {latestDiagnostic(record) ?? "No diagnostics"}
                  </p>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </Section>
  );
}

export { SettingsApplyRecordsPanel };
export type { SettingsApplyRecordsPanelProps };
