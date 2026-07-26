import {
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
  FieldLabel,
  FieldTitle,
  FormSection,
  Input,
  NativeSelect,
  NativeSelectOption,
  Switch,
  Textarea,
} from "@agh/ui";

import type { SandboxDraft, SandboxEnvPair } from "../lib/sandbox-profile-draft";

const PERSISTENCE_MODES: readonly { value: string; label: string }[] = [
  { value: "", label: "Provider default" },
  { value: "transient", label: "Transient" },
  { value: "reuse", label: "Reuse" },
  { value: "archive", label: "Archive" },
];

const DAYTONA_FIELDS: readonly { key: string; label: string; placeholder: string }[] = [
  { key: "api_url", label: "API URL", placeholder: "https://app.daytona.io/api" },
  { key: "target", label: "Target", placeholder: "default" },
  { key: "image", label: "Image", placeholder: "ghcr.io/compozy/agh-dev" },
  { key: "snapshot", label: "Snapshot", placeholder: "snapshot id" },
  { key: "class", label: "Class", placeholder: "medium" },
  { key: "auto_stop", label: "Auto stop", placeholder: "30m" },
  { key: "auto_archive", label: "Auto archive", placeholder: "24h" },
];

function serializePairs(pairs: readonly SandboxEnvPair[]): string {
  return pairs.map(pair => `${pair.key}=${pair.value}`).join("\n");
}

function parsePairs(raw: string): SandboxEnvPair[] {
  return raw.split("\n").flatMap(line => {
    const trimmed = line.trim();
    if (trimmed.length === 0) return [];
    const separator = trimmed.indexOf("=");
    if (separator < 0) return [{ key: trimmed, value: "" }];
    return [
      { key: trimmed.slice(0, separator).trim(), value: trimmed.slice(separator + 1).trim() },
    ];
  });
}

function serializeList(list: readonly string[] | undefined): string {
  return (list ?? []).join("\n");
}

function parseList(raw: string): string[] {
  return raw.split("\n").flatMap(line => {
    const trimmed = line.trim();
    return trimmed.length > 0 ? [trimmed] : [];
  });
}

export interface SandboxEditorAdvancedSectionProps {
  draft: SandboxDraft;
  /** Per-row non-secret `env` errors, keyed by draft index. */
  envErrors?: Record<number, string>;
  /** Per-row `secret_env` reference errors, keyed by draft index. */
  secretEnvErrors?: Record<number, string>;
  onChange: (updater: (draft: SandboxDraft) => SandboxDraft) => void;
}

/**
 * Advanced tier: lifecycle, environment, network policy, and Daytona.
 *
 * `secret_env` holds *references* (`env:NAME`, `vault:sandbox/...`), never
 * plaintext — the daemon validates that shape — so the references are editable
 * and safe to display. The Daytona block renders only for the Daytona backend.
 */
export function SandboxEditorAdvancedSection({
  draft,
  envErrors,
  secretEnvErrors,
  onChange,
}: SandboxEditorAdvancedSectionProps) {
  const isDaytona = draft.backend.trim() === "daytona";
  const envError = Object.values(envErrors ?? {})[0];
  const secretEnvError = Object.values(secretEnvErrors ?? {})[0];

  return (
    <>
      <FormSection
        data-testid="sandbox-editor-lifecycle"
        description="What happens to the sandbox between sessions."
        title="Isolation & lifecycle"
      >
        <Field>
          <FieldLabel htmlFor="sandbox-editor-persistence">Persistence</FieldLabel>
          <NativeSelect
            data-testid="sandbox-editor-persistence"
            id="sandbox-editor-persistence"
            onChange={event =>
              onChange(current => ({ ...current, persistence: event.target.value }))
            }
            value={draft.persistence}
          >
            {PERSISTENCE_MODES.map(option => (
              <NativeSelectOption key={option.value || "default"} value={option.value}>
                {option.label}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </Field>

        <Field>
          <FieldLabel htmlFor="sandbox-editor-runtime-root">Runtime root</FieldLabel>
          <Input
            className="font-mono"
            data-testid="sandbox-editor-runtime-root"
            id="sandbox-editor-runtime-root"
            onChange={event =>
              onChange(current => ({ ...current, runtime_root: event.target.value }))
            }
            placeholder="/workspace"
            value={draft.runtime_root}
          />
        </Field>

        <Field data-invalid={Boolean(envError)}>
          <FieldLabel htmlFor="sandbox-editor-env">Environment</FieldLabel>
          <FieldDescription>
            Non-secret <code className="font-mono">KEY=value</code> pairs, one per line.
          </FieldDescription>
          <Textarea
            aria-invalid={Boolean(envError)}
            data-testid="sandbox-editor-env"
            id="sandbox-editor-env"
            onChange={event =>
              onChange(current => ({ ...current, env: parsePairs(event.target.value) }))
            }
            placeholder="LOG_LEVEL=info"
            rows={3}
            value={serializePairs(draft.env)}
            variant="mono"
          />
          <FieldError data-testid="sandbox-editor-env-error">{envError}</FieldError>
        </Field>

        <Field data-invalid={Boolean(secretEnvError)}>
          <FieldLabel htmlFor="sandbox-editor-secret-env">Secret environment</FieldLabel>
          <FieldDescription>
            <code className="font-mono">KEY=env:NAME</code> or{" "}
            <code className="font-mono">KEY=vault:sandbox/…</code> references, one per line. The
            profile stores the reference; the value never passes through here.
          </FieldDescription>
          <Textarea
            aria-invalid={Boolean(secretEnvError)}
            data-testid="sandbox-editor-secret-env"
            id="sandbox-editor-secret-env"
            onChange={event =>
              onChange(current => ({ ...current, secretEnv: parsePairs(event.target.value) }))
            }
            placeholder="GITHUB_TOKEN=vault:sandbox/github"
            rows={3}
            value={serializePairs(draft.secretEnv)}
            variant="mono"
          />
          <FieldError data-testid="sandbox-editor-secret-env-error">{secretEnvError}</FieldError>
        </Field>
      </FormSection>

      <FormSection
        data-testid="sandbox-editor-network"
        description={
          isDaytona
            ? "Daytona supports public ingress. It does not enforce outbound or host allow and deny rules."
            : "Outbound and ingress rules enforced by the backend."
        }
        title="Network policy"
      >
        <Field orientation="horizontal">
          <FieldContent>
            <FieldTitle>Allow public ingress</FieldTitle>
            <FieldDescription>
              {isDaytona
                ? "Expose the Daytona sandbox publicly."
                : "Expose the sandbox publicly — only when the backend supports it."}
            </FieldDescription>
          </FieldContent>
          <Switch
            aria-label="Allow public ingress"
            checked={Boolean(draft.network.allow_public_ingress)}
            data-testid="sandbox-editor-allow-ingress"
            onCheckedChange={allow_public_ingress =>
              onChange(current => ({
                ...current,
                network: { ...current.network, allow_public_ingress },
              }))
            }
          />
        </Field>

        {isDaytona ? (
          <p
            className="text-form-hint text-subtle"
            data-testid="sandbox-editor-daytona-network-limitation"
          >
            Daytona does not enforce outbound or host allow and deny rules, so this profile will not
            send them.
          </p>
        ) : (
          <>
            <Field orientation="horizontal">
              <FieldContent>
                <FieldTitle>Allow outbound</FieldTitle>
                <FieldDescription>
                  Permit outbound connections, subject to the lists below.
                </FieldDescription>
              </FieldContent>
              <Switch
                aria-label="Allow outbound"
                checked={Boolean(draft.network.allow_outbound)}
                data-testid="sandbox-editor-allow-outbound"
                onCheckedChange={allow_outbound =>
                  onChange(current => ({
                    ...current,
                    network: { ...current.network, allow_outbound },
                  }))
                }
              />
            </Field>

            <div className="grid gap-3.5 md:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="sandbox-editor-allow-list">Allow list</FieldLabel>
                <Textarea
                  data-testid="sandbox-editor-allow-list"
                  id="sandbox-editor-allow-list"
                  onChange={event =>
                    onChange(current => ({
                      ...current,
                      network: { ...current.network, allow_list: parseList(event.target.value) },
                    }))
                  }
                  placeholder="api.github.com"
                  rows={3}
                  value={serializeList(draft.network.allow_list)}
                  variant="mono"
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="sandbox-editor-deny-list">Deny list</FieldLabel>
                <Textarea
                  data-testid="sandbox-editor-deny-list"
                  id="sandbox-editor-deny-list"
                  onChange={event =>
                    onChange(current => ({
                      ...current,
                      network: { ...current.network, deny_list: parseList(event.target.value) },
                    }))
                  }
                  placeholder="metadata.internal"
                  rows={3}
                  value={serializeList(draft.network.deny_list)}
                  variant="mono"
                />
              </Field>
            </div>
          </>
        )}
      </FormSection>

      {isDaytona ? (
        <FormSection
          data-testid="sandbox-editor-daytona"
          description="Cloud workspace parameters for this profile."
          title="Daytona workspace"
        >
          <div className="grid gap-3.5 md:grid-cols-2">
            {DAYTONA_FIELDS.map(field => (
              <Field key={field.key}>
                <FieldLabel htmlFor={`sandbox-editor-daytona-${field.key}`}>
                  {field.label}
                </FieldLabel>
                <Input
                  className="font-mono"
                  data-testid={`sandbox-editor-daytona-${field.key}`}
                  id={`sandbox-editor-daytona-${field.key}`}
                  onChange={event =>
                    onChange(current => ({
                      ...current,
                      daytona: { ...current.daytona, [field.key]: event.target.value },
                    }))
                  }
                  placeholder={field.placeholder}
                  value={(draft.daytona as Record<string, string | undefined>)[field.key] ?? ""}
                />
              </Field>
            ))}
          </div>
        </FormSection>
      ) : null}
    </>
  );
}
