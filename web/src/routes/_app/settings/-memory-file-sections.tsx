import { SettingsFieldRow, SettingsGroup, SettingsNumberInput } from "@/systems/settings";
import { Input, Switch } from "@agh/ui";
import {
  type DraftSectionProps,
  type ValidatedSectionProps,
  TEST_PREFIX,
} from "./-memory-settings-types";

export function SessionLedgerSection({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: ValidatedSectionProps) {
  return (
    <SettingsGroup title="Session ledger" description="forensic JSONL ledger materialization">
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-session-ledger-format`}
        label="Ledger format"
        description="jsonl is the only Slice 1 ledger format"
        control={
          <Input
            className="w-32 font-mono"
            data-testid={`${TEST_PREFIX}-session-ledger-format-input`}
            value={draft.session.ledger_format}
            placeholder="jsonl"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  session: { ...current.session, ledger_format: event.target.value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-session-events-purge-grace`}
        label="Events purge grace"
        description="Hold events.db rows this long before purging materialized ledgers"
        control={
          <Input
            className="w-32 font-mono"
            data-testid={`${TEST_PREFIX}-session-events-purge-grace-input`}
            value={draft.session.events_purge_grace}
            placeholder="24h"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  session: { ...current.session, events_purge_grace: event.target.value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-session-cold-archive-days`}
        label="Cold archive (days)"
        description="Move ledgers to cold archive after this many days"
        error={validationErrors.sessionColdArchive ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid={`${TEST_PREFIX}-session-cold-archive-days-input`}
            value={draft.session.cold_archive_days}
            onValidityChange={setValidationError("sessionColdArchive")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  session: { ...current.session, cold_archive_days: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-session-hard-delete-days`}
        label="Hard delete (days)"
        description="0 means never auto-delete; ledgers prune only via explicit CLI"
        error={validationErrors.sessionHardDelete ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid={`${TEST_PREFIX}-session-hard-delete-days-input`}
            value={draft.session.hard_delete_days}
            onValidityChange={setValidationError("sessionHardDelete")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  session: { ...current.session, hard_delete_days: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-session-max-archive-bytes`}
        label="Max archive bytes"
        description="Safety valve for cold archive size"
        error={validationErrors.sessionMaxArchive ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-40"
            data-testid={`${TEST_PREFIX}-session-max-archive-bytes-input`}
            value={draft.session.max_archive_bytes}
            onValidityChange={setValidationError("sessionMaxArchive")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  session: { ...current.session, max_archive_bytes: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-session-ledger-root`}
        label="Ledger root"
        description="Read-only daemon-managed ledger root"
        control={
          <Input
            readOnly
            className="w-full font-mono"
            data-testid={`${TEST_PREFIX}-session-ledger-root-input`}
            value={draft.session.ledger_root}
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-session-unbound-partition`}
        label="Unbound partition"
        description="Read-only directory for sessions without a workspace binding"
        control={
          <Input
            readOnly
            className="w-full font-mono"
            data-testid={`${TEST_PREFIX}-session-unbound-partition-input`}
            value={draft.session.unbound_partition}
          />
        }
      />
    </SettingsGroup>
  );
}

export function DailyLogsSection({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: ValidatedSectionProps) {
  return (
    <SettingsGroup title="Daily logs" description="rotation, dreaming window, and archival">
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-daily-max-bytes`}
        label="Max bytes per file"
        description="Daily log rotates when it crosses this byte budget"
        error={validationErrors.dailyMaxBytes ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-32"
            data-testid={`${TEST_PREFIX}-daily-max-bytes-input`}
            value={draft.daily.max_bytes}
            onValidityChange={setValidationError("dailyMaxBytes")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  daily: { ...current.daily, max_bytes: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-daily-max-lines`}
        label="Max lines per file"
        description="Hard line ceiling before rotation"
        error={validationErrors.dailyMaxLines ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-32"
            data-testid={`${TEST_PREFIX}-daily-max-lines-input`}
            value={draft.daily.max_lines}
            onValidityChange={setValidationError("dailyMaxLines")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  daily: { ...current.daily, max_lines: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-daily-sweep-hour`}
        label="Sweep hour"
        description="Local hour for the daily housekeeping sweep"
        error={validationErrors.dailySweepHour ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid={`${TEST_PREFIX}-daily-sweep-hour-input`}
            value={draft.daily.sweep_hour}
            onValidityChange={setValidationError("dailySweepHour")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  daily: { ...current.daily, sweep_hour: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-daily-dreaming-window`}
        label="Dreaming window (days)"
        description="How many days of daily logs feed dreaming candidates"
        error={validationErrors.dailyDreamingWindow ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid={`${TEST_PREFIX}-daily-dreaming-window-input`}
            value={draft.daily.dreaming_window}
            onValidityChange={setValidationError("dailyDreamingWindow")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  daily: { ...current.daily, dreaming_window: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-daily-cold-archive-days`}
        label="Cold archive (days)"
        description="Move daily logs to cold archive after this many days"
        error={validationErrors.dailyColdArchive ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid={`${TEST_PREFIX}-daily-cold-archive-days-input`}
            value={draft.daily.cold_archive_days}
            onValidityChange={setValidationError("dailyColdArchive")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  daily: { ...current.daily, cold_archive_days: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-daily-hard-delete-days`}
        label="Hard delete (days)"
        description="0 means never auto-delete; daily logs prune only via explicit CLI"
        error={validationErrors.dailyHardDelete ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid={`${TEST_PREFIX}-daily-hard-delete-days-input`}
            value={draft.daily.hard_delete_days}
            onValidityChange={setValidationError("dailyHardDelete")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  daily: { ...current.daily, hard_delete_days: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-daily-max-archive-bytes`}
        label="Max archive bytes"
        description="Safety valve cap for daily-log cold archive size"
        error={validationErrors.dailyMaxArchiveBytes ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-40"
            data-testid={`${TEST_PREFIX}-daily-max-archive-bytes-input`}
            value={draft.daily.max_archive_bytes}
            onValidityChange={setValidationError("dailyMaxArchiveBytes")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  daily: { ...current.daily, max_archive_bytes: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-daily-rotate-format`}
        label="Rotate format"
        description="Read-only daemon-managed daily-log rotation pattern"
        control={
          <Input
            readOnly
            className="w-full font-mono"
            data-testid={`${TEST_PREFIX}-daily-rotate-format-input`}
            value={draft.daily.rotate_format}
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-daily-archive-path`}
        label="Archive path"
        description="Read-only relative path for daily-log cold archive"
        control={
          <Input
            readOnly
            className="w-full font-mono"
            data-testid={`${TEST_PREFIX}-daily-archive-path-input`}
            value={draft.daily.archive_path}
          />
        }
      />
    </SettingsGroup>
  );
}

export function FileCapsSection({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: ValidatedSectionProps) {
  return (
    <SettingsGroup title="File caps" description="MEMORY.md projection ceilings">
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-file-max-lines`}
        label="Max lines"
        description="Soft cap for the MEMORY.md projection"
        error={validationErrors.fileMaxLines ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid={`${TEST_PREFIX}-file-max-lines-input`}
            value={draft.file.max_lines}
            onValidityChange={setValidationError("fileMaxLines")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  file: { ...current.file, max_lines: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-file-max-bytes`}
        label="Max bytes"
        description="Hard byte budget for the MEMORY.md projection"
        error={validationErrors.fileMaxBytes ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-32"
            data-testid={`${TEST_PREFIX}-file-max-bytes-input`}
            value={draft.file.max_bytes}
            onValidityChange={setValidationError("fileMaxBytes")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  file: { ...current.file, max_bytes: value },
                };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}

export function WorkspaceIdentitySection({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingsGroup title="Workspace identity" description=".agh/workspace.toml lifecycle">
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-workspace-toml-path`}
        label="Workspace toml path"
        description="Informational; validation locks this to <workspace>/.agh/workspace.toml"
        control={
          <Input
            readOnly
            className="w-full font-mono"
            data-testid={`${TEST_PREFIX}-workspace-toml-path-input`}
            value={draft.workspace.toml_path}
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-workspace-auto-create`}
        label="Auto-create workspace.toml"
        description="Create <workspace>/.agh/workspace.toml on first touch"
        control={
          <Switch
            data-testid={`${TEST_PREFIX}-workspace-auto-create-switch`}
            checked={draft.workspace.auto_create}
            onCheckedChange={checked =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  workspace: { ...current.workspace, auto_create: checked },
                };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}
