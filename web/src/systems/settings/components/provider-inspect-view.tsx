import { RefreshCw } from "lucide-react";
import type { ReactNode } from "react";

import {
  Button,
  Empty,
  Eyebrow,
  MetadataList,
  MetadataListRow,
  MonoId,
  Pill,
  type PillTone,
  Spinner,
  Time,
} from "@compozy/ui";

import { providerAuthStateLabel, providerAuthSummary } from "../lib/provider-copy";
import { getProviderStateView } from "../lib/provider-state";
import type { SettingsProviderEntry } from "../types";
import { ProviderLoginDescriptorView } from "./provider-login-descriptor";
import { ProviderStatusLabel } from "./provider-status-label";
import { SettingsAdvancedFold } from "./settings-advanced-fold";
import { SettingsSourceBadge } from "./settings-source-badge";
import {
  modelRefreshStateTone,
  type ProviderModelSourceStatus,
  useProviderModelStatus,
  useRefreshProviderModels,
} from "@/systems/model-catalog";

interface ProviderInspectViewProps {
  provider: SettingsProviderEntry;
  onRefreshCatalog: () => void;
  /** Runs the status summary's next step (opens the Configure tab). */
  onAction?: () => void;
}

/**
 * Provider overview: a status summary that answers "is it working, and what do
 * I do?", with the raw configuration behind a closed "Technical details" fold.
 */
export function ProviderInspectView({ provider, onAction }: ProviderInspectViewProps) {
  const state = getProviderStateView(provider);
  const ready = state.label === "installed";
  const defaultModel = provider.settings.models?.default ?? null;
  const showLogin = state.label === "needs-sign-in" && Boolean(provider.auth_status?.login);

  return (
    <div className="flex flex-col gap-5">
      <section
        className="flex flex-col gap-3 rounded-lg bg-canvas shadow-card p-4"
        data-testid="provider-detail-summary"
      >
        <div className="flex flex-col gap-1">
          <ProviderStatusLabel
            data-testid="provider-detail-summary-status"
            label={state.display}
            ready={ready}
            tone={state.tone}
          />
          {state.hint ? <p className="text-small-body text-muted">{state.hint}</p> : null}
        </div>
        <MetadataList>
          <MetadataListRow label="Sign-in">{providerAuthSummary(provider)}</MetadataListRow>
          {defaultModel ? (
            <MetadataListRow label="Default model">
              <span className="text-fg" data-testid="inspect-default-model">
                {defaultModel}
              </span>
            </MetadataListRow>
          ) : null}
        </MetadataList>
        {showLogin && provider.auth_status?.login ? (
          <ProviderLoginDescriptorView
            login={provider.auth_status.login}
            testId="inspect-login-descriptor"
          />
        ) : null}
        {!ready && onAction ? (
          <Button
            className="w-fit"
            data-testid="provider-detail-summary-action"
            onClick={onAction}
            size="sm"
            type="button"
            variant="outline"
          >
            {state.cta.label}
          </Button>
        ) : null}
      </section>

      <SettingsAdvancedFold bare data-testid="provider-detail-technical" label="Technical details">
        <ProviderTechnicalDetails provider={provider} showLogin={!showLogin} />
      </SettingsAdvancedFold>
    </div>
  );
}

function ProviderTechnicalDetails({
  provider,
  showLogin,
}: {
  provider: SettingsProviderEntry;
  showLogin: boolean;
}) {
  return (
    <div className="flex flex-col gap-5 pb-1">
      <RuntimeSection provider={provider} />
      <CuratedModelsSection provider={provider} />
      <AuthenticationSection provider={provider} showLogin={showLogin} />
      <CredentialsSection provider={provider} />

      <InspectSection id="source" label="Defined in">
        <SettingsSourceBadge
          data-testid="inspect-source"
          source={provider.source_metadata.effective_source}
          shadowed={provider.source_metadata.shadowed_sources ?? []}
        />
      </InspectSection>

      <InspectSection id="catalog" label="Model list">
        <CatalogList providerId={provider.name} enabled={provider.command_available} />
      </InspectSection>
    </div>
  );
}

function RuntimeSection({ provider }: { provider: SettingsProviderEntry }) {
  const harness = provider.settings.harness ?? null;
  const runtime = provider.settings.runtime_provider ?? null;
  return (
    <InspectSection id="runtime" label="How it runs">
      <MetadataListRow label="Name">
        <MonoId preserveCase value={provider.name} />
      </MetadataListRow>
      <MetadataListRow label="Command" valueProps={{ "data-testid": "inspect-command" }}>
        {provider.settings.command ? (
          <code className="font-mono text-mono-id break-all text-fg">
            {provider.settings.command}
          </code>
        ) : (
          "—"
        )}
      </MetadataListRow>
      {harness ? (
        <MetadataListRow label="Adapter" valueProps={{ "data-testid": "inspect-harness" }}>
          <MonoId preserveCase value={harness} />
          {runtime && runtime !== harness ? (
            <span className="ml-2 text-subtle">
              via <MonoId preserveCase value={runtime} />
            </span>
          ) : null}
        </MetadataListRow>
      ) : null}
    </InspectSection>
  );
}

function CuratedModelsSection({ provider }: { provider: SettingsProviderEntry }) {
  const curated = (provider.settings.models?.curated ?? []).flatMap(model =>
    model.id ? [model.id] : []
  );
  if (curated.length === 0) return null;
  return (
    <InspectSection id="models" label="Models">
      <MetadataListRow label="Available">
        <ul className="flex flex-wrap gap-1.5" data-testid="inspect-curated-models">
          {curated.map(id => (
            <li key={id}>
              <Pill mono size="xs" tone="neutral">
                {id}
              </Pill>
            </li>
          ))}
        </ul>
      </MetadataListRow>
    </InspectSection>
  );
}

function AuthenticationSection({
  provider,
  showLogin,
}: {
  provider: SettingsProviderEntry;
  showLogin: boolean;
}) {
  const authStatus = provider.auth_status;
  return (
    <InspectSection id="authentication" label="Sign-in">
      <MetadataListRow label="Mode">
        <code className="font-mono text-mono-id text-fg" data-testid="inspect-auth-mode">
          {provider.settings.auth_mode ?? "—"}
        </code>
      </MetadataListRow>
      <MetadataListRow label="Env policy">
        <MonoId preserveCase value={provider.settings.env_policy ?? "—"} />
      </MetadataListRow>
      <MetadataListRow label="Home policy">
        <MonoId preserveCase value={provider.settings.home_policy ?? "—"} />
      </MetadataListRow>
      {authStatus?.state ? (
        <MetadataListRow label="Status">
          <span className="flex flex-col gap-1" data-testid="inspect-auth-status">
            <span className="text-fg">{providerAuthStateLabel(authStatus.state)}</span>
            {authStatus.message ? <span>{authStatus.message}</span> : null}
          </span>
        </MetadataListRow>
      ) : null}
      {showLogin && authStatus?.login ? (
        <MetadataListRow label="Login app">
          <ProviderLoginDescriptorView login={authStatus.login} testId="inspect-login-descriptor" />
        </MetadataListRow>
      ) : null}
    </InspectSection>
  );
}

function CredentialsSection({ provider }: { provider: SettingsProviderEntry }) {
  const credentials = provider.credentials ?? [];
  const credentialSlots = provider.settings.credential_slots ?? [];
  if (credentials.length === 0 && credentialSlots.length === 0) return null;
  return (
    <InspectSection
      id="credentials"
      label={`Keys (${credentials.length || credentialSlots.length})`}
    >
      <CredentialList slots={credentialSlots} credentials={credentials} />
    </InspectSection>
  );
}

function InspectSection({
  id,
  label,
  children,
}: {
  id: string;
  label: string;
  children: ReactNode;
}) {
  return (
    <section className="flex flex-col gap-2.5" data-section={id}>
      <Eyebrow className="text-subtle">{label}</Eyebrow>
      <MetadataList>{children}</MetadataList>
    </section>
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
        const stateLabel: string = required && !present ? "Missing" : present ? "Set" : "Optional";
        const stateTone: PillTone = required && !present ? "warning" : "neutral";
        return (
          <li
            key={slot.name}
            className="flex flex-col gap-1.5 rounded-lg bg-sunken px-3 py-2.5"
            data-testid={`inspect-credential-${slot.name}`}
          >
            <div className="flex items-center justify-between gap-2">
              <MonoId preserveCase value={slot.name} />
              <Pill tone={stateTone}>{stateLabel}</Pill>
            </div>
            <MetadataList>
              <MetadataListRow label="Variable">
                <MonoId preserveCase value={slot.target_env} />
              </MetadataListRow>
              <MetadataListRow label="Stored in">
                <MonoId preserveCase value={slot.secret_ref} />
              </MetadataListRow>
            </MetadataList>
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
      <p className="text-form-hint text-subtle" data-testid="inspect-catalog-disabled">
        The model list updates once the app is installed.
      </p>
    );
  }

  if (statusQuery.isLoading) {
    return (
      <div className="flex items-center gap-2 text-form-hint text-subtle">
        <Spinner className="size-3" />
        <span>Loading model list…</span>
      </div>
    );
  }

  const sources = statusQuery.data?.sources ?? [];
  const refreshError = errorMessage(refreshMutation.error);
  const queryError = errorMessage(statusQuery.error);
  const refreshing = refreshMutation.isPending || statusQuery.isFetching;

  return (
    <div className="flex flex-col gap-2.5" data-testid="inspect-catalog">
      {queryError ? <p className="text-form-hint text-danger">{queryError}</p> : null}
      {sources.length === 0 && !queryError ? (
        <Empty
          data-testid="inspect-catalog-empty"
          fill={false}
          size="compact"
          title="No model list yet"
        />
      ) : (
        <ul className="flex flex-col gap-1.5">
          {sources.map(source => (
            <CatalogRow key={source.source_id} source={source} />
          ))}
        </ul>
      )}
      {refreshError ? (
        <p className="text-form-hint text-danger" data-testid="inspect-catalog-refresh-error">
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
        {refreshMutation.isPending ? (
          <Spinner aria-hidden="true" className="size-3" />
        ) : (
          <RefreshCw aria-hidden="true" className="size-3" />
        )}
        {refreshing ? "Refreshing…" : "Refresh model list"}
      </Button>
    </div>
  );
}

function CatalogRow({ source }: { source: ProviderModelSourceStatus }) {
  const timestamp = source.last_success?.trim() || source.last_refresh?.trim() || undefined;
  return (
    <li className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-2 rounded-md bg-sunken px-3 py-2">
      <div className="flex min-w-0 flex-col gap-0.5">
        <MonoId preserveCase value={source.source_id} />
        {timestamp ? (
          <span className="flex items-center gap-1 text-form-hint text-subtle">
            Updated <Time iso={timestamp} mode="relative" />
          </span>
        ) : null}
      </div>
      <div className="flex flex-col items-end gap-1">
        <span className="flex flex-wrap items-center gap-1.5">
          <Pill mono tone={modelRefreshStateTone(source.refresh_state)}>
            {source.refresh_state}
          </Pill>
          {source.stale ? <Pill tone="warning">Out of date</Pill> : null}
        </span>
        <span className="text-form-hint text-muted tabular-nums">{source.row_count} models</span>
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
