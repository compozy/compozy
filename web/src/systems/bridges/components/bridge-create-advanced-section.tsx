import { Waypoints } from "lucide-react";

import {
  Field,
  FieldContent,
  FieldHeader,
  FieldTitle,
  FormSection,
  HelpTip,
  NativeSelect,
  NativeSelectOption,
  Switch,
  Textarea,
} from "@compozy/ui";

import {
  describeBridgeDmPolicy,
  describeBridgeProviderConfigSchema,
} from "../lib/bridge-formatters";
import type { BridgeCreateDraft, BridgeProvider } from "../types";
import { BridgeDeliveryFields, BridgeRoutingFields } from "./bridge-delivery-fields";

export interface BridgeCreateAdvancedSectionProps {
  draft: BridgeCreateDraft;
  onDraftChange: (draft: BridgeCreateDraft) => void;
  provider: BridgeProvider | undefined;
  providerConfigError?: string;
}

/**
 * Advanced tier: optional policies from the bridge request contract.
 *
 * `provider_config` stays a free-form JSON object because the provider catalog
 * only reports a schema *hint* (`{schema, version}`), not field descriptors.
 * Secrets never belong here — the contract rejects secret-shaped JSON.
 */
export function BridgeCreateAdvancedSection({
  draft,
  onDraftChange,
  provider,
  providerConfigError,
}: BridgeCreateAdvancedSectionProps) {
  const configSchema = describeBridgeProviderConfigSchema(provider?.config_schema);
  return (
    <FormSection
      data-testid="bridge-create-section-routing"
      icon={Waypoints}
      title="Routing & delivery"
    >
      <Field>
        <FieldHeader>
          <FieldTitle>DM policy</FieldTitle>
          <HelpTip label="About DM policy">
            {describeBridgeDmPolicy(draft.dmPolicy === "" ? undefined : draft.dmPolicy)}
          </HelpTip>
        </FieldHeader>
        <NativeSelect
          aria-label="Direct message policy"
          data-testid="bridge-dm-policy-select"
          onChange={event =>
            onDraftChange({
              ...draft,
              dmPolicy: event.target.value as BridgeCreateDraft["dmPolicy"],
            })
          }
          value={draft.dmPolicy}
        >
          <NativeSelectOption value="">Use provider default</NativeSelectOption>
          <NativeSelectOption value="open">Open</NativeSelectOption>
          <NativeSelectOption value="allowlist">Allowlist</NativeSelectOption>
          <NativeSelectOption value="pairing">Pairing</NativeSelectOption>
        </NativeSelect>
      </Field>

      <BridgeRoutingFields
        onChange={routingPolicy => onDraftChange({ ...draft, routingPolicy })}
        testIdPrefix="bridge"
        value={draft.routingPolicy}
      />

      <BridgeDeliveryFields
        onChange={deliveryDefaults => onDraftChange({ ...draft, deliveryDefaults })}
        testIdPrefix="bridge"
        value={draft.deliveryDefaults}
      />

      <Field orientation="horizontal">
        <FieldContent>
          <FieldHeader>
            <FieldTitle>Suppress bridge notifications</FieldTitle>
            <HelpTip label="About suppress bridge notifications">
              Prevent notification fanout for deliveries from this bridge.
            </HelpTip>
          </FieldHeader>
        </FieldContent>
        <Switch
          aria-label="Suppress bridge notifications"
          checked={draft.notificationSuppress}
          data-testid="bridge-create-notification-suppress"
          onCheckedChange={notificationSuppress =>
            onDraftChange({ ...draft, notificationSuppress })
          }
        />
      </Field>

      <Field>
        <FieldHeader>
          <FieldTitle>Provider configuration</FieldTitle>
          <HelpTip label="About provider configuration">
            Non-secret JSON for provider settings such as tenant identifiers or mode flags.{" "}
            {configSchema}
          </HelpTip>
        </FieldHeader>
        <Textarea
          aria-invalid={Boolean(providerConfigError)}
          aria-label="Provider configuration JSON"
          className="min-h-32 font-mono text-xs"
          data-testid="bridge-provider-config-input"
          onChange={event => onDraftChange({ ...draft, providerConfigText: event.target.value })}
          placeholder={`{\n  "mode": "bot"\n}`}
          spellCheck={false}
          value={draft.providerConfigText}
        />
        {providerConfigError ? (
          <p className="text-small-body text-danger" data-testid="bridge-provider-config-error">
            {providerConfigError}
          </p>
        ) : null}
      </Field>
    </FormSection>
  );
}
