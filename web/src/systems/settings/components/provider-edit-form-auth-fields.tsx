import { KeyRound, ShieldOff, TerminalSquare } from "lucide-react";

import { Alert, AlertDescription, Eyebrow, FormSection, Input, Pill, RadioCard } from "@agh/ui";

import { withProviderAuthMode } from "../lib/provider-draft";
import type { ProviderAuthMode, ProviderDraft, SettingsProviderEntry } from "../types";
import type { ProviderDraftChange } from "./provider-edit-form";
import { ProviderCredentialFields } from "./provider-edit-form-credential-fields";
import { ModalSettingsFieldRow } from "./settings-field-row";

interface ProviderAuthFieldsProps {
  mode: "create" | "edit";
  draft: ProviderDraft;
  entry: SettingsProviderEntry | null;
  onChange: ProviderDraftChange;
}

const AUTH_CARDS: ReadonlyArray<{
  value: ProviderAuthMode;
  title: string;
  description: string;
  badge: string;
  icon: typeof KeyRound;
  testId: string;
}> = [
  {
    value: "native_cli",
    title: "Native CLI",
    description: "The provider owns login and credential state. AGH never asks for keys.",
    badge: "Provider-owned",
    icon: TerminalSquare,
    testId: "settings-providers-editor-auth-mode-native_cli",
  },
  {
    value: "bound_secret",
    title: "Bound secret",
    description: "AGH injects the declared credential slots at launch, from the vault.",
    badge: "AGH-managed",
    icon: KeyRound,
    testId: "settings-providers-editor-auth-mode-bound_secret",
  },
  {
    value: "none",
    title: "None",
    description: "The provider launches with no AGH-managed credentials.",
    badge: "Unauthenticated",
    icon: ShieldOff,
    testId: "settings-providers-editor-auth-mode-none",
  },
];

/**
 * Auth ownership and everything it governs.
 *
 * The mode is a security boundary, not a disclosure: AGH may offer credential
 * inputs only under `bound_secret`, and under `native_cli` it may do no more
 * than surface the provider-owned login command (`internal/CLAUDE.md`
 * § Provider auth boundary). The gate is therefore mount/unmount, never
 * disabled fields — a disabled credential input still says AGH wants the key.
 */
export function ProviderAuthFields({ mode, draft, entry, onChange }: ProviderAuthFieldsProps) {
  return (
    <FormSection
      data-testid="settings-providers-editor-auth"
      description="AGH asks for credentials only under a bound-secret contract."
      title="Who owns authentication?"
    >
      <div
        aria-label="Authentication ownership"
        className="grid gap-2 sm:grid-cols-3"
        data-testid="settings-providers-editor-auth-mode"
        role="radiogroup"
      >
        {AUTH_CARDS.map(card => (
          <RadioCard
            data-testid={card.testId}
            description={
              <span className="flex flex-col items-start gap-1.5">
                {card.description}
                <Pill size="xs" tone={card.value === "bound_secret" ? "info" : "neutral"}>
                  {card.badge}
                </Pill>
              </span>
            }
            icon={card.icon}
            key={card.value}
            onSelect={() => onChange(current => withProviderAuthMode(current, card.value))}
            selected={draft.auth_mode === card.value}
            title={card.title}
            titleClassName="min-w-0 flex-1 truncate"
          />
        ))}
      </div>

      {draft.auth_mode === "native_cli" ? (
        <ProviderNativeAuthFields
          draft={draft}
          entry={mode === "edit" ? entry : null}
          onChange={onChange}
        />
      ) : null}

      {draft.auth_mode === "bound_secret" ? (
        <ProviderCredentialFields draft={draft} entry={entry} mode={mode} onChange={onChange} />
      ) : null}

      {draft.auth_mode === "none" ? (
        <Alert data-testid="settings-providers-editor-auth-none-note" variant="info">
          <AlertDescription className="text-xs">
            No credentials are injected into the provider subprocess. The safety rationale (
            <code className="font-mono">none_security</code>) stays as configured — this editor does
            not change it.
          </AlertDescription>
        </Alert>
      ) : null}
    </FormSection>
  );
}

interface ProviderNativeAuthFieldsProps {
  draft: ProviderDraft;
  entry: SettingsProviderEntry | null;
  onChange: ProviderDraftChange;
}

/**
 * Provider-owned login: the two commands AGH may run on the operator's behalf,
 * plus the last reported status. No credential input belongs here.
 */
function ProviderNativeAuthFields({ draft, entry, onChange }: ProviderNativeAuthFieldsProps) {
  const authStatus = entry?.auth_status ?? null;

  return (
    <>
      {authStatus?.state ? (
        <Alert data-testid="settings-providers-editor-auth-status" variant="info">
          <AlertDescription className="flex flex-col gap-1 text-xs">
            <span>
              Native CLI status: <code className="font-mono">{authStatus.state}</code>
            </span>
            {authStatus.message ? <span className="text-muted">{authStatus.message}</span> : null}
          </AlertDescription>
        </Alert>
      ) : null}

      <ModalSettingsFieldRow
        control={
          <Input
            className="w-72 font-mono"
            data-testid="settings-providers-editor-auth-status-command-input"
            onChange={event =>
              onChange(current => ({ ...current, auth_status_command: event.target.value }))
            }
            placeholder="codex auth status"
            value={draft.auth_status_command}
          />
        }
        data-testid="settings-providers-editor-auth-status-command"
        description="Provider-owned command used for auth diagnostics."
        label={
          <>
            Status command
            <Eyebrow className="ml-1.5 text-subtle">optional</Eyebrow>
          </>
        }
      />

      <ModalSettingsFieldRow
        control={
          <Input
            className="w-72 font-mono"
            data-testid="settings-providers-editor-auth-login-command-input"
            onChange={event =>
              onChange(current => ({ ...current, auth_login_command: event.target.value }))
            }
            placeholder="codex login"
            value={draft.auth_login_command}
          />
        }
        data-testid="settings-providers-editor-auth-login-command"
        description="Provider-owned command opened by provider auth login. AGH never stores what it produces."
        label={
          <>
            Login command
            <Eyebrow className="ml-1.5 text-subtle">optional</Eyebrow>
          </>
        }
      />
    </>
  );
}
