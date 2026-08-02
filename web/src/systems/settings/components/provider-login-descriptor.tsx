import { Pill, type PillTone } from "@compozy/ui";

import type { SettingsProviderLoginDescriptor } from "../types";

interface ProviderLoginDescriptorViewProps {
  login: SettingsProviderLoginDescriptor | null;
  testId: string;
}

const PRESENCE_VIEW: Record<
  SettingsProviderLoginDescriptor["presence"],
  { label: string; tone: PillTone }
> = {
  present: { label: "Available", tone: "success" },
  missing: { label: "Not found", tone: "warning" },
  unknown: { label: "Not checked", tone: "neutral" },
};

const ACTION_LABEL: Record<
  NonNullable<SettingsProviderLoginDescriptor["recommended_action"]>,
  string
> = {
  install_cli: "Install the provider CLI.",
  login: "Sign in with the provider CLI.",
  bind_secret: "Bind the required credential.",
  retry: "Retry the authentication check.",
  inspect: "Inspect the provider status.",
  no_retry: "Review the failure before retrying.",
};

/** Safe public projection of a write-only provider login command. */
export function ProviderLoginDescriptorView({ login, testId }: ProviderLoginDescriptorViewProps) {
  if (!login?.configured) {
    return (
      <Pill data-testid={testId} size="xs" tone="neutral">
        Not configured
      </Pill>
    );
  }

  const presence = PRESENCE_VIEW[login.presence];
  const executable = login.executable?.trim() || "Configured CLI";
  const action = login.recommended_action ? ACTION_LABEL[login.recommended_action] : null;

  return (
    <div className="flex min-w-0 flex-col gap-1.5" data-testid={testId}>
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <code
          className="max-w-full truncate font-mono text-small-body text-fg"
          data-testid={`${testId}-executable`}
        >
          {executable}
        </code>
        <Pill size="xs" tone={presence.tone}>
          {presence.label}
        </Pill>
        {login.source === "auth_login_command" ? (
          <Pill mono size="xs" tone="neutral">
            write-only config
          </Pill>
        ) : null}
      </div>
      {action ? (
        <span className="text-form-hint text-muted" data-testid={`${testId}-action`}>
          Next: {action}
        </span>
      ) : null}
    </div>
  );
}
