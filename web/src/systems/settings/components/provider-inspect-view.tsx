import { RefreshCw } from "lucide-react";

import { Button, Eyebrow, Pill, type PillTone, Spinner, Time } from "@agh/ui";

import {
  modelRefreshStateTone,
  useProviderModelStatus,
  useRefreshProviderModels,
  type ProviderModelSourceStatus,
} from "@/systems/model-catalog";

import type { SettingsProviderEntry } from "../types";
import { SettingsSourceBadge } from "./settings-source-badge";

interface ProviderInspectViewProps {
  provider: SettingsProviderEntry;
  onRefreshCatalog: () => void;
}

export function ProviderInspectView({ provider }: ProviderInspectViewProps) {
  const credentials = provider.credentials ?? [];
  const credentialSlots = provider.settings.credential_slots ?? [];
  const curated = (provider.settings.models?.curated ?? []).flatMap(model =>
    model.id ? [model.id] : []
  );
  const defaultModel = provider.settings.models?.default ?? null;
  const hasCredentials = credentials.length > 0 || credentialSlots.length > 0;

  return (
    <div className="flex flex-col gap-6">
      <InspectSection label="Runtime">
        <Row label="Command">
          <CommandBlock value={provider.settings.command ?? null} />
        </Row>
        <Row label="Harness">
          <HarnessValue
            harness={provider.settings.harness ?? null}
            runtime={provider.settings.runtime_provider ?? null}
          />
        </Row>
      </InspectSection>

      {defaultModel || curated.length > 0 ? (
        <InspectSection label="Models">
          {defaultModel ? (
            <Row label="Default">
              <code
                className="font-mono text-small-body text-fg"
                data-testid="inspect-default-model"
              >
                {defaultModel}
              </code>
            </Row>
          ) : null}
          {curated.length > 0 ? (
            <Row label="Curated" align="start">
              <ul className="flex flex-wrap gap-1.5" data-testid="inspect-curated-models">
                {curated.map(id => (
                  <li
                    key={id}
                    className="rounded-mono-badge bg-canvas-soft px-1.5 py-0.5 font-mono text-mono-id text-muted"
                  >
                    {id}
                  </li>
                ))}
              </ul>
            </Row>
          ) : null}
        </InspectSection>
      ) : null}

      <InspectSection label="Authentication">
        <Row label="Mode">
          <code className="font-mono text-small-body text-fg" data-testid="inspect-auth-mode">
            {provider.settings.auth_mode ?? "—"}
          </code>
        </Row>
        <Row label="Env policy">
          <code className="font-mono text-small-body text-muted">
            {provider.settings.env_policy ?? "—"}
          </code>
        </Row>
        <Row label="Home policy">
          <code className="font-mono text-small-body text-muted">
            {provider.settings.home_policy ?? "—"}
          </code>
        </Row>
        {provider.auth_status?.state ? (
          <Row label="Status" align="start">
            <AuthStatusValue
              state={provider.auth_status.state}
              message={provider.auth_status.message}
            />
          </Row>
        ) : null}
      </InspectSection>

      {hasCredentials ? (
        <InspectSection label={`Credentials (${credentials.length || credentialSlots.length})`}>
          <CredentialList slots={credentialSlots} credentials={credentials} />
        </InspectSection>
      ) : null}

      <InspectSection label="Source">
        <SettingsSourceBadge
          data-testid="inspect-source"
          source={provider.source_metadata.effective_source}
          shadowed={provider.source_metadata.shadowed_sources ?? []}
        />
      </InspectSection>

      <InspectSection label="Catalog">
        <CatalogList providerId={provider.name} enabled={provider.command_available} />
      </InspectSection>
    </div>
  );
}

function InspectSection({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <section className="flex flex-col gap-2.5" data-section={label.toLowerCase()}>
      <Eyebrow className="text-subtle">{label}</Eyebrow>
      <div className="flex flex-col gap-2">{children}</div>
    </section>
  );
}

function Row({
  label,
  children,
  align = "center",
}: {
  label: string;
  children: React.ReactNode;
  align?: "center" | "start";
}) {
  return (
    <div
      className={`grid grid-cols-[8rem_minmax(0,1fr)] gap-3 ${
        align === "start" ? "items-start" : "items-center"
      }`}
    >
      <span className="text-xs text-subtle">{label}</span>
      <div className="min-w-0">{children}</div>
    </div>
  );
}

function CommandBlock({ value }: { value: string | null }) {
  if (!value) {
    return <span className="text-xs text-subtle">—</span>;
  }
  return (
    <code
      className="block rounded-sm bg-canvas-soft px-2 py-1.5 font-mono text-xs text-fg break-all"
      data-testid="inspect-command"
    >
      {value}
    </code>
  );
}

function HarnessValue({ harness, runtime }: { harness: string | null; runtime: string | null }) {
  if (!harness) {
    return <span className="text-xs text-subtle">—</span>;
  }
  return (
    <span className="flex flex-wrap items-center gap-2" data-testid="inspect-harness">
      <code className="font-mono text-small-body text-fg">{harness}</code>
      {runtime && runtime !== harness ? (
        <span className="text-xs text-subtle">
          via <code className="font-mono">{runtime}</code>
        </span>
      ) : null}
    </span>
  );
}

function AuthStatusValue({ state, message }: { state: string; message?: string }) {
  return (
    <span className="flex flex-col gap-1">
      <code className="font-mono text-small-body text-fg" data-testid="inspect-auth-status">
        {state}
      </code>
      {message ? <span className="text-xs text-muted">{message}</span> : null}
    </span>
  );
}

type CredentialSlot = NonNullable<
  NonNullable<SettingsProviderEntry["settings"]["credential_slots"]>
>[number];

type CredentialStatus = NonNullable<SettingsProviderEntry["credentials"]>[number];

function CredentialList({
  slots,
  credentials,
}: {
  slots: readonly CredentialSlot[];
  credentials: readonly CredentialStatus[];
}) {
  const byName = new Map<string, CredentialStatus>(credentials.map(item => [item.name, item]));
  const items = slots.length > 0 ? slots : credentials.map(toSlotShape);

  return (
    <ul className="flex flex-col gap-2">
      {items.map(slot => {
        const status = byName.get(slot.name);
        const present = status?.present ?? false;
        const required = slot.required ?? status?.required ?? false;
        const stateLabel: string =
          required && !present ? "missing" : present ? "bound" : "optional";
        const stateTone: PillTone =
          required && !present ? "warning" : present ? "success" : "neutral";
        return (
          <li
            key={slot.name}
            className="flex flex-col gap-1.5 rounded-md bg-canvas-soft px-3 py-2.5"
            data-testid={`inspect-credential-${slot.name}`}
          >
            <div className="flex items-center justify-between gap-2">
              <code className="font-mono text-small-body text-fg">{slot.name}</code>
              <Pill tone={stateTone} mono>
                {stateLabel}
              </Pill>
            </div>
            <dl className="grid grid-cols-[5rem_minmax(0,1fr)] gap-x-3 gap-y-1 text-xs text-muted">
              <dt className="text-subtle">target env</dt>
              <dd>
                <code className="font-mono">{slot.target_env}</code>
              </dd>
              <dt className="text-subtle">secret ref</dt>
              <dd>
                <code className="font-mono break-all">{slot.secret_ref}</code>
              </dd>
            </dl>
          </li>
        );
      })}
    </ul>
  );
}

function toSlotShape(credential: CredentialStatus): CredentialSlot {
  return {
    name: credential.name,
    target_env: credential.target_env,
    secret_ref: credential.secret_ref,
    kind: credential.kind,
    required: credential.required,
  };
}

function CatalogList({ providerId, enabled }: { providerId: string; enabled: boolean }) {
  const statusQuery = useProviderModelStatus({ providerId, enabled });
  const refreshMutation = useRefreshProviderModels();

  if (!enabled) {
    return (
      <p className="text-xs text-subtle" data-testid="inspect-catalog-disabled">
        Catalog refresh resumes once the provider binary is available.
      </p>
    );
  }

  if (statusQuery.isLoading) {
    return (
      <div className="flex items-center gap-2 text-xs text-subtle">
        <Spinner className="size-3" />
        <span>Loading catalog status…</span>
      </div>
    );
  }

  const sources = statusQuery.data?.sources ?? [];
  const refreshError = errorMessage(refreshMutation.error);
  const queryError = errorMessage(statusQuery.error);
  const refreshing = refreshMutation.isPending || statusQuery.isFetching;

  return (
    <div className="flex flex-col gap-2.5" data-testid="inspect-catalog">
      {queryError ? <p className="text-xs text-danger">{queryError}</p> : null}
      {sources.length === 0 && !queryError ? (
        <p className="text-xs text-subtle" data-testid="inspect-catalog-empty">
          No catalog sources reporting yet.
        </p>
      ) : (
        <ul className="flex flex-col gap-1.5">
          {sources.map(source => (
            <CatalogRow key={source.source_id} source={source} />
          ))}
        </ul>
      )}
      {refreshError ? (
        <p className="text-xs text-danger" data-testid="inspect-catalog-refresh-error">
          {refreshError}
        </p>
      ) : null}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="w-fit"
        onClick={() => refreshMutation.mutate({ providerId, force: true })}
        disabled={refreshing}
        data-testid="inspect-catalog-refresh"
      >
        <RefreshCw
          aria-hidden="true"
          className={refreshMutation.isPending ? "size-3 animate-spin" : "size-3"}
        />
        {refreshing ? "Refreshing…" : "Refresh catalog"}
      </Button>
    </div>
  );
}

function CatalogRow({ source }: { source: ProviderModelSourceStatus }) {
  const timestamp = source.last_success?.trim() || source.last_refresh?.trim() || undefined;
  return (
    <li className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-2 rounded-sm bg-canvas-soft px-3 py-2">
      <div className="flex min-w-0 flex-col gap-0.5">
        <code className="truncate font-mono text-small-body text-fg">{source.source_id}</code>
        {timestamp ? (
          <span className="flex items-center gap-1 text-xs text-subtle">
            <Eyebrow className="text-subtle">refreshed</Eyebrow>
            <Time iso={timestamp} mode="relative" />
          </span>
        ) : null}
      </div>
      <div className="flex flex-col items-end gap-1">
        <span className="flex flex-wrap items-center gap-1.5">
          <Pill mono tone={modelRefreshStateTone(source.refresh_state)}>
            {source.refresh_state}
          </Pill>
          {source.stale ? (
            <Pill mono tone="warning">
              stale
            </Pill>
          ) : null}
        </span>
        <span className="text-xs text-muted tabular-nums">{source.row_count} rows</span>
      </div>
    </li>
  );
}

function errorMessage(error: unknown): string | null {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return null;
}
