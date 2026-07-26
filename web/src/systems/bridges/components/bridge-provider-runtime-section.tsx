import { AlertCircle } from "lucide-react";

import { CodeBlock, DataSurface, Eyebrow, MetadataList, Section } from "@agh/ui";

import { describeBridgeProviderConfigSchema } from "../lib/bridge-formatters";
import type { BridgeSetupProjection } from "../lib/bridge-setup";
import type { BridgeProvider, BridgeSecretBinding } from "../types";
import { BridgeSecretSlotCard } from "./bridge-secret-slot-card";

const TILE_CLASS =
  "grid-cols-1 items-start rounded-md border border-line bg-canvas-soft px-4 py-3 sm:grid-cols-[7.5rem_1fr] sm:items-center";
const TERM_CLASS = "text-muted";
const VALUE_CLASS = "min-w-0 break-words text-small-body text-fg";

interface BridgeProviderRuntimeSectionProps {
  bindingsByName: Map<string, BridgeSecretBinding>;
  checksBySecretSlot: BridgeSetupProjection["checksBySecretSlot"];
  isProviderLoading: boolean;
  isSecretBindingPending: boolean;
  isSecretBindingsLoading: boolean;
  onDeleteSecretBinding?: (bindingName: string) => void;
  onSaveSecretBinding?: (bindingName: string) => void;
  onSecretDraftChange?: (bindingName: string, value: string) => void;
  provider?: BridgeProvider;
  providerError: Error | null;
  providerConfig: string;
  secretBindingsError: Error | null;
  secretInputValues: Record<string, string>;
}

export function BridgeProviderRuntimeSection({
  bindingsByName,
  checksBySecretSlot,
  isProviderLoading,
  isSecretBindingPending,
  isSecretBindingsLoading,
  onDeleteSecretBinding,
  onSaveSecretBinding,
  onSecretDraftChange,
  provider,
  providerError,
  providerConfig,
  secretBindingsError,
  secretInputValues,
}: BridgeProviderRuntimeSectionProps) {
  const providerState = isProviderLoading ? "loading" : providerError ? "error" : "ready";
  const bindingsState = isSecretBindingsLoading
    ? "loading"
    : secretBindingsError
      ? "error"
      : "ready";

  return (
    <Section label="Provider runtime">
      <DataSurface data-testid="bridge-provider-runtime-state" state={providerState}>
        <DataSurface.Loading
          data-testid="bridge-provider-runtime-loading"
          label="Loading provider runtime"
          size="sm"
        />
        <DataSurface.Error
          data-testid="bridge-provider-runtime-unavailable"
          description={providerError?.message ?? "Provider metadata could not be loaded."}
          icon={AlertCircle}
          title="Provider runtime unavailable"
        />
        <DataSurface.Content>
          <MetadataList className="grid gap-3 sm:grid-cols-2">
            <MetadataList.Row
              className={TILE_CLASS}
              label="Manifest schema"
              termProps={{ className: TERM_CLASS }}
              valueProps={{ className: VALUE_CLASS }}
            >
              {describeBridgeProviderConfigSchema(provider?.config_schema)}
            </MetadataList.Row>
            <MetadataList.Row
              className={TILE_CLASS}
              label="Secret slots"
              termProps={{ className: TERM_CLASS }}
              valueProps={{ className: VALUE_CLASS }}
            >
              {provider?.secret_slots?.length ? provider.secret_slots.length : "None declared"}
            </MetadataList.Row>
          </MetadataList>

          {provider?.description ? (
            <p className="mt-3 rounded-md border border-line bg-canvas-soft px-4 py-3 text-small-body leading-relaxed text-muted">
              {provider.description}
            </p>
          ) : null}

          {provider?.secret_slots?.length ? (
            <DataSurface className="mt-3" state={bindingsState}>
              <DataSurface.Loading
                data-testid="bridge-secret-bindings-loading"
                label="Loading secret bindings"
                size="sm"
              />
              <DataSurface.Error
                data-testid="bridge-secret-bindings-unavailable"
                description={secretBindingsError?.message ?? "Secret bindings could not be loaded."}
                icon={AlertCircle}
                title="Secret bindings unavailable"
              />
              <DataSurface.Content
                className="space-y-3 scroll-mt-4"
                data-testid="bridge-detail-secret-slots"
                id="bridge-secret-slots"
                tabIndex={-1}
              >
                {provider.secret_slots.map(slot => (
                  <BridgeSecretSlotCard
                    binding={bindingsByName.get(slot.name)}
                    checks={checksBySecretSlot[slot.name] ?? []}
                    inputValue={secretInputValues[slot.name] ?? ""}
                    isSecretBindingPending={isSecretBindingPending}
                    key={slot.name}
                    onDelete={onDeleteSecretBinding}
                    onDraftChange={onSecretDraftChange}
                    onSave={onSaveSecretBinding}
                    slot={slot}
                  />
                ))}
              </DataSurface.Content>
            </DataSurface>
          ) : null}

          <div className="mt-3">
            <Eyebrow className="mb-2 block text-muted">Provider config</Eyebrow>
            {providerConfig ? (
              <CodeBlock
                code={providerConfig}
                copyable={false}
                data-testid="bridge-detail-provider-config"
                showPrompt={false}
              />
            ) : (
              <p className="rounded-md border border-line bg-canvas-soft px-4 py-3 text-small-body leading-relaxed text-muted">
                No provider runtime config stored for this bridge.
              </p>
            )}
          </div>
        </DataSurface.Content>
      </DataSurface>
    </Section>
  );
}
