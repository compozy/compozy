import {
  Eyebrow,
  FormSection,
  Input,
  NativeSelect,
  NativeSelectOption,
  RequiredMark,
  Textarea,
} from "@agh/ui";

import type { ProviderDraft } from "../types";
import type { ProviderDraftChange } from "./provider-edit-form";
import { ModalSettingsFieldRow } from "./settings-field-row";

interface ProviderRuntimeFieldsProps {
  draft: ProviderDraft;
  onChange: ProviderDraftChange;
}

const HARNESSES = ["acp", "pi_acp"] as const;
const ENV_POLICIES = ["filtered", "isolated"] as const;
const HOME_POLICIES = ["operator", "isolated"] as const;

/**
 * Advanced tier: how the provider is launched and which models it offers.
 *
 * `RuntimeSelector` is forbidden here — this surface configures a provider, it
 * does not choose one (`MODAL-STANDARD.md` § Forbidden drift).
 */
export function ProviderRuntimeFields({ draft, onChange }: ProviderRuntimeFieldsProps) {
  const runtimeProviderRequired = draft.harness === "pi_acp";

  return (
    <FormSection
      data-testid="settings-providers-editor-runtime"
      description="Provider-specific overrides stay out of the primary path."
      title="Runtime & models"
    >
      <ModalSettingsFieldRow
        control={
          <NativeSelect
            className="w-40 font-mono"
            data-testid="settings-providers-editor-harness-input"
            onChange={event => onChange(current => ({ ...current, harness: event.target.value }))}
            value={draft.harness}
          >
            {HARNESSES.map(option => (
              <NativeSelectOption key={option} value={option}>
                {option}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        }
        data-testid="settings-providers-editor-harness"
        description="Runtime adapter used to launch the provider."
        label={
          <>
            Harness
            <RequiredMark />
          </>
        }
      />

      <ModalSettingsFieldRow
        control={
          <Input
            className="w-56 font-mono"
            data-testid="settings-providers-editor-runtime-provider-input"
            onChange={event =>
              onChange(current => ({ ...current, runtime_provider: event.target.value }))
            }
            placeholder="openrouter"
            value={draft.runtime_provider}
          />
        }
        data-testid="settings-providers-editor-runtime-provider"
        description="Downstream provider id used by the selected harness. Required when using pi_acp."
        label={
          <>
            Runtime provider
            {runtimeProviderRequired ? (
              <RequiredMark />
            ) : (
              <Eyebrow className="ml-1.5 text-subtle">pi</Eyebrow>
            )}
          </>
        }
      />

      <ModalSettingsFieldRow
        control={
          <Input
            className="w-56 font-mono"
            data-testid="settings-providers-editor-transport-input"
            onChange={event => onChange(current => ({ ...current, transport: event.target.value }))}
            placeholder="openai"
            value={draft.transport}
          />
        }
        data-testid="settings-providers-editor-transport"
        description="Provider API family or Pi models override transport."
        label={
          <>
            Transport
            <Eyebrow className="ml-1.5 text-subtle">optional</Eyebrow>
          </>
        }
      />

      <ModalSettingsFieldRow
        control={
          <Input
            className="w-72 font-mono"
            data-testid="settings-providers-editor-base-url-input"
            onChange={event => onChange(current => ({ ...current, base_url: event.target.value }))}
            placeholder="https://openrouter.ai/api/v1"
            type="url"
            value={draft.base_url}
          />
        }
        data-testid="settings-providers-editor-base-url"
        description="Custom API base URL for Pi-backed model overrides."
        label={
          <>
            Base URL
            <Eyebrow className="ml-1.5 text-subtle">optional</Eyebrow>
          </>
        }
      />

      <ModalSettingsFieldRow
        control={
          <Input
            className="w-56 font-mono"
            data-testid="settings-providers-editor-model-input"
            onChange={event =>
              onChange(current => ({ ...current, model_default: event.target.value }))
            }
            placeholder="Leave blank to use the provider default"
            value={draft.model_default}
          />
        }
        data-testid="settings-providers-editor-model"
        description="Sent to the provider when an agent does not specify one."
        label={
          <>
            Default model
            <Eyebrow className="ml-1.5 text-subtle">optional</Eyebrow>
          </>
        }
      />

      <ModalSettingsFieldRow
        control={
          <Textarea
            className="min-h-24 w-72 font-mono text-xs"
            data-testid="settings-providers-editor-curated-models-input"
            onChange={event =>
              onChange(current => ({ ...current, curated_models: event.target.value }))
            }
            placeholder="One model ID per line"
            value={draft.curated_models}
          />
        }
        data-testid="settings-providers-editor-curated-models"
        description="Provider-scoped model IDs stored under models.curated."
        label={
          <>
            Curated models
            <Eyebrow className="ml-1.5 text-subtle">optional</Eyebrow>
          </>
        }
      />

      <ModalSettingsFieldRow
        control={
          <NativeSelect
            className="w-40 font-mono"
            data-testid="settings-providers-editor-env-policy-input"
            onChange={event =>
              onChange(current => ({ ...current, env_policy: event.target.value }))
            }
            value={draft.env_policy}
          >
            {ENV_POLICIES.map(option => (
              <NativeSelectOption key={option} value={option}>
                {option}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        }
        data-testid="settings-providers-editor-env-policy"
        description="Daemon environment inheritance for the provider subprocess."
        label={
          <>
            Env policy
            <RequiredMark />
          </>
        }
      />

      <ModalSettingsFieldRow
        control={
          <NativeSelect
            className="w-40 font-mono"
            data-testid="settings-providers-editor-home-policy-input"
            onChange={event =>
              onChange(current => ({ ...current, home_policy: event.target.value }))
            }
            value={draft.home_policy}
          >
            {HOME_POLICIES.map(option => (
              <NativeSelectOption key={option} value={option}>
                {option}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        }
        data-testid="settings-providers-editor-home-policy"
        description="Provider CLI state location policy."
        label={
          <>
            Home policy
            <RequiredMark />
          </>
        }
      />
    </FormSection>
  );
}
