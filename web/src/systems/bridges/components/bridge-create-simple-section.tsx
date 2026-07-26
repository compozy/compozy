import { Fingerprint } from "lucide-react";

import { Field, FieldLabel, FormSection, Input, RequiredMark } from "@agh/ui";

import type { BridgeCreateDraft, BridgeProvider } from "../types";
import { BridgeCreateProviderStep } from "./bridge-create-provider-step";

export interface BridgeCreateSimpleSectionProps {
  draft: BridgeCreateDraft;
  onDraftChange: (draft: BridgeCreateDraft) => void;
  onSelectProvider: (key: string) => void;
  providerSelectionDisabled?: boolean;
  providers: BridgeProvider[];
  supportsManifest: boolean;
}

/**
 * Simple tier: where the bridge connects and how it identifies itself.
 *
 * Provider and account are fixed at creation — `UpdateBridgeRequest` omits
 * `platform` and `extension_name` — so the choice belongs here, never behind
 * Advanced.
 */
export function BridgeCreateSimpleSection({
  draft,
  onDraftChange,
  onSelectProvider,
  providerSelectionDisabled = false,
  providers,
  supportsManifest,
}: BridgeCreateSimpleSectionProps) {
  return (
    <>
      <BridgeCreateProviderStep
        onSelect={onSelectProvider}
        disabled={providerSelectionDisabled}
        providers={providers}
        selectedProviderKey={draft.selectedProviderKey}
        supportsManifest={supportsManifest}
      />

      <FormSection
        data-testid="bridge-create-section-identity"
        description="How this bridge appears in lists and receipts."
        icon={Fingerprint}
        title="Identity"
      >
        <Field>
          <FieldLabel htmlFor="bridge-display-name-input">
            Display name
            <RequiredMark />
          </FieldLabel>
          <Input
            data-testid="bridge-display-name-input"
            id="bridge-display-name-input"
            onChange={event => onDraftChange({ ...draft, displayName: event.target.value })}
            placeholder="Support operations"
            value={draft.displayName}
          />
        </Field>
      </FormSection>
    </>
  );
}
