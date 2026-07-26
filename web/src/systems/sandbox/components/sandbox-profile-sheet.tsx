import { Boxes, X } from "lucide-react";
import type { ReactNode } from "react";

import { Button, Eyebrow, Pill, Sheet, SheetContent } from "@agh/ui";

import { SettingsSourceBadge, type SettingsSandboxEntry } from "@/systems/settings";
import {
  sandboxBackendLabel,
  sandboxBackendTone,
  sandboxIsDeletable,
  sandboxOrDash,
  sandboxUsageLabel,
} from "../lib/sandbox-labels";

export interface SandboxProfileSheetProps {
  entry: SettingsSandboxEntry | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onRequestDelete: (entry: SettingsSandboxEntry) => void;
  onRequestEdit?: (entry: SettingsSandboxEntry) => void;
}

export function SandboxProfileSheet({
  entry,
  open,
  onOpenChange,
  onRequestDelete,
  onRequestEdit,
}: SandboxProfileSheetProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        className="grid w-full grid-rows-[auto_1fr_auto] gap-0 p-0 sm:max-w-[32.5rem]"
        data-testid="sandbox-profile-sheet"
        showCloseButton={false}
        side="right"
      >
        {entry ? (
          <>
            <SheetHead entry={entry} onClose={() => onOpenChange(false)} />
            <div className="min-h-0 overflow-y-auto px-5 py-4.5">
              <SheetTiles entry={entry} />
              <NetworkSection entry={entry} />
              <DaytonaSection entry={entry} />
              <EnvSection entry={entry} />
              <UsageSection entry={entry} />
              <QuietNote />
              {onRequestEdit ? (
                <div className="mb-4.5">
                  <Button
                    data-testid="sandbox-profile-sheet-edit"
                    onClick={() => onRequestEdit(entry)}
                    size="sm"
                    type="button"
                    variant="outline"
                  >
                    Edit profile
                  </Button>
                </div>
              ) : null}
              {sandboxIsDeletable(entry) ? (
                <SheetDangerZone onRequestDelete={() => onRequestDelete(entry)} />
              ) : null}
            </div>
            <footer
              className="flex items-center gap-2.5 border-t border-line-soft px-5 py-3"
              data-testid="sandbox-profile-sheet-foot"
            >
              <span className="min-w-0 truncate font-mono text-mono-id text-faint">
                agh config get sandboxes.{entry.name}.backend
              </span>
            </footer>
          </>
        ) : null}
      </SheetContent>
    </Sheet>
  );
}

function SheetHead({ entry, onClose }: { entry: SettingsSandboxEntry; onClose: () => void }) {
  const backend = entry.profile.backend;
  return (
    <header className="flex items-start gap-3 border-b border-line px-5 py-4.5">
      <span
        aria-hidden="true"
        className="grid size-9 shrink-0 place-items-center rounded-md bg-accent-tint text-accent-strong"
      >
        <Boxes className="size-4" />
      </span>
      <div className="min-w-0 flex-1">
        <Eyebrow className="text-subtle">Sandbox profile</Eyebrow>
        <div className="mt-0.5 flex min-w-0 flex-wrap items-center gap-2">
          <h2
            className="mt-0.5 text-item-title font-medium tracking-tight text-fg-strong"
            data-testid="sandbox-profile-sheet-title"
            id="sandbox-profile-sheet-title"
          >
            <span className="font-mono">{entry.name}</span>
          </h2>
          <Pill mono size="sm" tone={sandboxBackendTone(backend)}>
            {backend}
          </Pill>
        </div>
        <p
          className="mt-1.5 truncate text-form-hint text-muted"
          data-testid="sandbox-profile-sheet-sub"
        >
          {sandboxBackendLabel(backend)}
        </p>
      </div>
      <Button
        aria-label="Close"
        className="shrink-0 text-muted hover:bg-row-hover hover:text-fg-strong"
        data-testid="sandbox-profile-sheet-close"
        onClick={onClose}
        size="icon-sm"
        type="button"
        variant="ghost"
      >
        <X aria-hidden="true" className="size-3.5" />
      </Button>
    </header>
  );
}

function SheetTiles({ entry }: { entry: SettingsSandboxEntry }) {
  const profile = entry.profile;
  return (
    <div className="mb-4 grid grid-cols-2 gap-2.5" data-testid="sandbox-profile-sheet-tiles">
      <Tile label="Backend">
        <Pill mono size="sm" tone={sandboxBackendTone(profile.backend)}>
          {profile.backend}
        </Pill>
      </Tile>
      <Tile label="Sync mode">
        <span className="font-mono text-mono-id text-muted">
          {sandboxOrDash(profile.sync_mode)}
        </span>
      </Tile>
      <Tile label="Persistence">
        <span className="font-mono text-mono-id text-muted">
          {sandboxOrDash(profile.persistence)}
        </span>
      </Tile>
      <Tile label="Runtime root">
        <span className="font-mono text-mono-id text-muted">
          {sandboxOrDash(profile.runtime_root)}
        </span>
      </Tile>
    </div>
  );
}

function Tile({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="rounded-md border border-line-soft bg-input-fill px-3.5 py-2.5">
      <Eyebrow className="mb-1.5 text-subtle">{label}</Eyebrow>
      <div className="flex min-w-0 items-center gap-1.5">{children}</div>
    </div>
  );
}

function NetworkSection({ entry }: { entry: SettingsSandboxEntry }) {
  const network = entry.profile.network;
  if (!network) return null;

  return (
    <section className="mb-4.5" data-testid="sandbox-profile-sheet-network">
      <Eyebrow className="mb-2.5 text-subtle">Network</Eyebrow>
      <div className="overflow-hidden rounded-md border border-line-soft">
        <KvRow label="Outbound">
          {network.allow_outbound ? (
            <Pill size="sm" tone="success">
              allowed
            </Pill>
          ) : (
            <Pill size="sm" tone="warning">
              blocked
            </Pill>
          )}
        </KvRow>
        <KvRow label="Public ingress">
          {network.allow_public_ingress ? (
            <Pill size="sm" tone="warning">
              allowed
            </Pill>
          ) : (
            <Pill size="sm" tone="neutral">
              off
            </Pill>
          )}
        </KvRow>
        <KvRow label="Allow list">
          <KeyChips values={network.allow_list ?? []} />
        </KvRow>
        <KvRow label="Deny list">
          <KeyChips values={network.deny_list ?? []} />
        </KvRow>
      </div>
    </section>
  );
}

function DaytonaSection({ entry }: { entry: SettingsSandboxEntry }) {
  const daytona = entry.profile.daytona;
  if (entry.profile.backend !== "daytona" || !daytona) return null;

  return (
    <section className="mb-4.5" data-testid="sandbox-profile-sheet-daytona">
      <Eyebrow className="mb-2.5 text-subtle">Daytona</Eyebrow>
      <div className="overflow-hidden rounded-md border border-line-soft">
        {daytona.image ? (
          <KvRow label="Image">
            <code className="font-mono text-mono-id text-muted">{daytona.image}</code>
          </KvRow>
        ) : null}
        {daytona.target ? (
          <KvRow label="Target">
            <code className="font-mono text-mono-id text-muted">{daytona.target}</code>
          </KvRow>
        ) : null}
        {daytona.auto_stop ? (
          <KvRow label="Auto stop">
            <code className="font-mono text-mono-id text-muted">{daytona.auto_stop}</code>
          </KvRow>
        ) : null}
        {daytona.auto_archive ? (
          <KvRow label="Auto archive">
            <code className="font-mono text-mono-id text-muted">{daytona.auto_archive}</code>
          </KvRow>
        ) : null}
        {daytona.class ? (
          <KvRow label="Class">
            <code className="font-mono text-mono-id text-muted">{daytona.class}</code>
          </KvRow>
        ) : null}
        {daytona.snapshot ? (
          <KvRow label="Snapshot">
            <code className="font-mono text-mono-id text-muted">{daytona.snapshot}</code>
          </KvRow>
        ) : null}
      </div>
    </section>
  );
}

function EnvSection({ entry }: { entry: SettingsSandboxEntry }) {
  const env = entry.profile.env;
  const secretEnv = entry.profile.secret_env;
  const envKeys = env ? Object.keys(env) : [];
  const secretKeys = secretEnv ? Object.keys(secretEnv) : [];
  if (envKeys.length === 0 && secretKeys.length === 0) return null;

  return (
    <section className="mb-4.5" data-testid="sandbox-profile-sheet-env">
      <Eyebrow className="mb-2.5 text-subtle">Environment</Eyebrow>
      <div className="overflow-hidden rounded-md border border-line-soft">
        {envKeys.length > 0 ? (
          <KvRow label="Env">
            <KeyChips values={envKeys} />
          </KvRow>
        ) : null}
        {secretKeys.length > 0 ? (
          <KvRow label="Secret env">
            <KeyChips tone="info" values={secretKeys} />
          </KvRow>
        ) : null}
      </div>
      {secretKeys.length > 0 ? (
        <p className="mt-1.5 text-form-hint leading-normal text-subtle">
          Secret bindings are references and never return in plaintext.
        </p>
      ) : null}
    </section>
  );
}

function UsageSection({ entry }: { entry: SettingsSandboxEntry }) {
  return (
    <section className="mb-4.5" data-testid="sandbox-profile-sheet-usage">
      <Eyebrow className="mb-2.5 text-subtle">Usage</Eyebrow>
      <div className="overflow-hidden rounded-md border border-line-soft">
        <KvRow label="Workspaces">
          <span className="text-form-label text-muted">
            {sandboxUsageLabel(entry.workspace_usage_count)} reference this profile
          </span>
        </KvRow>
        <KvRow label="Source">
          <div className="flex min-w-0 flex-col items-end gap-1">
            <SettingsSourceBadge
              data-testid="sandbox-profile-sheet-source"
              shadowed={entry.source_metadata.shadowed_sources ?? []}
              source={entry.source_metadata.effective_source}
            />
            <span className="font-mono text-mono-id text-faint">[sandboxes.{entry.name}]</span>
          </div>
        </KvRow>
      </div>
    </section>
  );
}

function QuietNote() {
  return (
    <p
      className="mb-4.5 flex items-start gap-2 rounded-md border border-line-soft bg-canvas-soft px-3 py-2.5 text-form-hint leading-snug text-subtle"
      data-testid="sandbox-profile-sheet-quiet-note"
    >
      <span>
        <code className="font-mono text-inline-code">daytona</code>,{" "}
        <code className="font-mono text-inline-code">network</code>,{" "}
        <code className="font-mono text-inline-code">env</code>, and{" "}
        <code className="font-mono text-inline-code">secret_env</code> are edited in the profile
        editor&apos;s Advanced tier. This sheet inspects; it never writes.
      </span>
    </p>
  );
}

function SheetDangerZone({ onRequestDelete }: { onRequestDelete: () => void }) {
  return (
    <section data-testid="sandbox-profile-sheet-danger">
      <div className="flex items-center justify-between gap-3 rounded-md border border-danger/20 bg-danger-tint px-3.5 py-3">
        <div className="min-w-0">
          <b className="block text-small-body font-medium text-danger">Delete profile</b>
          <p className="mt-0.5 text-form-hint leading-snug text-muted">
            Removing the overlay stops making this profile selectable for new workspaces.
          </p>
        </div>
        <Button
          data-testid="sandbox-profile-sheet-delete"
          onClick={onRequestDelete}
          size="sm"
          type="button"
          variant="destructive"
        >
          Delete
        </Button>
      </div>
    </section>
  );
}

function KvRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-3 border-b border-line-soft px-3.5 py-2.5 last:border-b-0">
      <span className="shrink-0 text-form-label text-subtle">{label}</span>
      <div className="min-w-0 text-right">{children}</div>
    </div>
  );
}

function KeyChips({ values, tone = "neutral" }: { values: string[]; tone?: "neutral" | "info" }) {
  if (values.length === 0) {
    return <span className="text-form-hint text-faint">--</span>;
  }
  return (
    <span className="flex flex-wrap justify-end gap-1">
      {values.map(value => (
        <Pill key={value} mono size="sm" tone={tone}>
          {value}
        </Pill>
      ))}
    </span>
  );
}
