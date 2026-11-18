import { Button, MetadataList, MetadataListRow, Spinner, Time } from "@compozy/ui";

import { useCheckProviderSignIn } from "../hooks/use-check-provider-sign-in";
import { providerCanDisableRuntimeMCP } from "../lib/mcp-runtime-view-model";
import { providerAuthSummary } from "../lib/provider-copy";
import { getProviderStateView, providerShowsLogin } from "../lib/provider-state";
import type { SettingsProviderEntry } from "../types";
import { MCPRuntimeModeSwitch } from "./mcp-runtime-mode-switch";
import { ProviderLoginDescriptorView } from "./provider-login-descriptor";
import { ProviderStatusLabel } from "./provider-status-label";

/**
 * The provider's status summary: state, when it was last checked, sign-in and default model, the
 * runtime MCP switch, the sign-in steps, and the one next action.
 */
export function ProviderStatusSummary({
  provider,
  onAction,
}: {
  provider: SettingsProviderEntry;
  /** Runs the next step when it opens the editor (the Configure tab). */
  onAction?: () => void;
}) {
  const state = getProviderStateView(provider);
  const checkSignIn = useCheckProviderSignIn();
  const runAction =
    state.cta.intent === "check" ? () => checkSignIn.mutate(provider.name) : onAction;
  const login = providerShowsLogin(provider) ? provider.auth_status?.login : undefined;
  return (
    <section
      className="flex flex-col gap-3 rounded-lg bg-sunken p-4"
      data-testid="provider-detail-summary"
    >
      <div className="flex flex-col gap-1">
        <ProviderStatusLabel
          data-testid="provider-detail-summary-status"
          label={state.display}
          tone={state.tone}
        />
        <ProviderStatusNotes
          checkedAt={provider.auth_status?.last_probe_at}
          error={checkSignIn.error?.message}
          hint={state.hint}
        />
      </div>
      <ProviderStatusFacts provider={provider} />
      <ProviderRuntimeMCPSwitch provider={provider} />
      {login ? (
        <ProviderLoginDescriptorView login={login} testId="inspect-login-descriptor" />
      ) : null}
      {state.label !== "installed" && runAction ? (
        <Button
          className="w-fit"
          data-testid="provider-detail-summary-action"
          disabled={checkSignIn.isPending}
          onClick={runAction}
          size="sm"
          type="button"
          variant="secondary"
        >
          {checkSignIn.isPending ? <Spinner aria-hidden="true" className="size-3.5" /> : null}
          {state.cta.label}
        </Button>
      ) : null}
    </section>
  );
}

function ProviderStatusNotes({
  hint,
  checkedAt,
  error,
}: {
  hint?: string | null;
  checkedAt?: string | null;
  error?: string;
}) {
  return (
    <>
      {hint ? <p className="text-small-body text-muted-foreground">{hint}</p> : null}
      {checkedAt ? (
        <p
          className="text-small-body text-muted-foreground"
          data-testid="provider-detail-checked-at"
        >
          Checked <Time iso={checkedAt} mode="relative" />
        </p>
      ) : null}
      {error ? (
        <p className="text-small-body text-destructive" role="alert">
          {error}
        </p>
      ) : null}
    </>
  );
}

function ProviderStatusFacts({ provider }: { provider: SettingsProviderEntry }) {
  const defaultModel = provider.settings.models?.default ?? null;
  return (
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
  );
}

function ProviderRuntimeMCPSwitch({ provider }: { provider: SettingsProviderEntry }) {
  if (!providerCanDisableRuntimeMCP(provider.name)) return null;
  const displayName = provider.settings.display_name?.trim() || provider.name;
  return (
    <MCPRuntimeModeSwitch
      className="items-start"
      data-testid="provider-detail-runtime-mcp"
      label={`Use ${displayName}'s own MCP servers in CompozyOS sessions`}
      mode={provider.settings.runtime_mcp ?? "load"}
      provider={provider.name}
      runtimeName={displayName}
    />
  );
}
