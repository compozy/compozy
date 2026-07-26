import { Link2 } from "lucide-react";

import {
  Alert,
  AlertDescription,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  EntityModeToolbar,
  ImmutableIdentity,
  type EntityMode,
} from "@agh/ui";

import { ScopeSelector, type WorkspaceCommandSelectOption } from "@/systems/workspace";

import { parseBridgeProviderConfig } from "../lib/bridge-drafts";
import { findBridgeProviderByKey, isBridgeProviderSelectable } from "../lib/bridge-formatters";
import { missingRequiredBridgeSecretSlots } from "../lib/bridge-secret-slot-submission";
import type { BridgeCreateDraft, BridgeProvider } from "../types";
import { BridgeCreateAdvancedSection } from "./bridge-create-advanced-section";
import { BridgeCreateSecretSlots } from "./bridge-create-secret-slots";
import { BridgeCreateSimpleSection } from "./bridge-create-simple-section";
import {
  BridgeManifestHandoff,
  type BridgeManifestCommittedState,
} from "./bridge-manifest-handoff";

/**
 * Phase-two state after `POST /api/bridges` committed: the bridge exists, so the
 * dialog can no longer be cancelled back into nothing, and unbound slots have to
 * be retried against a real id.
 */
export interface BridgeSecretRecoveryState {
  bridgeId: string;
  bound: string[];
  failures: Record<string, string>;
  /** The provider captured with the committed bridge; the mutable draft cannot replace it. */
  provider: BridgeProvider;
}

interface BridgeCreateDialogProps {
  activeWorkspaceId?: string | null;
  activeWorkspaceName?: string | null;
  draft: BridgeCreateDraft;
  isPending: boolean;
  manifestState?: BridgeManifestCommittedState | null;
  mode?: EntityMode;
  onModeChange?: (mode: EntityMode) => void;
  onDraftChange: (draft: BridgeCreateDraft) => void;
  onOpenChange: (open: boolean) => void;
  onSubmit: () => void;
  open: boolean;
  providers: BridgeProvider[];
  secretRecovery?: BridgeSecretRecoveryState | null;
  supportsManifest?: boolean;
}

export function BridgeCreateDialog({
  activeWorkspaceId,
  activeWorkspaceName,
  draft,
  isPending,
  manifestState,
  mode = "simple",
  onModeChange,
  onDraftChange,
  onOpenChange,
  onSubmit,
  open,
  providers,
  secretRecovery = null,
  supportsManifest = false,
}: BridgeCreateDialogProps) {
  const committedManifestState = supportsManifest ? manifestState : null;
  const selectedProvider =
    secretRecovery?.provider ?? findBridgeProviderByKey(providers, draft.selectedProviderKey);
  const providerConfigError = parseBridgeProviderConfig(draft.providerConfigText).error;
  const providerSelectable = Boolean(
    selectedProvider && isBridgeProviderSelectable(selectedProvider)
  );
  // A manifest provider hands the credentials over only after the remote app is
  // created from the manifest, so its slots cannot gate the create call.
  const missingRequiredSlots = supportsManifest
    ? []
    : missingRequiredBridgeSecretSlots(selectedProvider?.secret_slots, draft.secretSlotValues);
  const scopeReady = draft.scope === "global" || Boolean(activeWorkspaceId);
  const canSubmit =
    providerSelectable &&
    scopeReady &&
    Boolean(draft.displayName.trim()) &&
    !providerConfigError &&
    missingRequiredSlots.length === 0;

  const workspaceOptions: WorkspaceCommandSelectOption[] =
    activeWorkspaceId && activeWorkspaceName
      ? [{ id: activeWorkspaceId, name: activeWorkspaceName }]
      : [];

  const handleOpenChange = (next: boolean) => {
    if (!next && isPending) return;
    onOpenChange(next);
  };

  const selectProvider = (key: string) => {
    const provider = findBridgeProviderByKey(providers, key);
    onDraftChange({
      ...draft,
      displayName:
        !draft.displayName.trim() || draft.displayName.trim() === selectedProvider?.display_name
          ? (provider?.display_name ?? draft.displayName)
          : draft.displayName,
      // Never carry a value typed for one provider's slot into another's.
      secretSlotValues: {},
      selectedProviderKey: key,
    });
  };

  if (committedManifestState) {
    return (
      <Dialog onOpenChange={handleOpenChange} open={open}>
        <DialogContent
          className={`grid-rows-[auto_minmax(0,1fr)] text-fg ${dialogShellClass("lg")}`}
          data-testid="bridge-create-dialog"
          showCloseButton={false}
          unframed
        >
          <EntityDialogHeader
            description="Create the Slack app from this manifest, then bind its credentials from the bridge detail."
            eyebrow="Catalog · Bridge"
            icon={Link2}
            onClose={() => handleOpenChange(false)}
            title="Set up Slack app"
          />
          <EntityDialogBody>
            <BridgeManifestHandoff state={committedManifestState} />
          </EntityDialogBody>
        </DialogContent>
      </Dialog>
    );
  }

  const recoveryFailures = secretRecovery?.failures ?? {};
  const failedSlotNames = Object.keys(recoveryFailures);
  const recoveryReady = failedSlotNames.every(
    name => (draft.secretSlotValues[name]?.trim().length ?? 0) > 0
  );

  return (
    <Dialog onOpenChange={handleOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("lg")}`}
        data-testid="bridge-create-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={
            <>
              A bridge connects a chat platform to your agents.{" "}
              <b className="font-medium text-muted">Its identity is permanent</b> — provider and
              account are fixed at creation; secrets stay write-only.
            </>
          }
          eyebrow="Catalog · Bridge"
          icon={Link2}
          onClose={isPending ? undefined : () => handleOpenChange(false)}
          title="Create bridge"
        />

        <EntityModeToolbar
          mode={mode}
          onModeChange={onModeChange ?? (() => undefined)}
          testIdPrefix="bridge-create"
          trailing={
            <ScopeSelector
              disabled={isPending || Boolean(secretRecovery)}
              onScopeChange={scope => onDraftChange({ ...draft, scope })}
              onWorkspaceChange={() => undefined}
              scope={draft.scope}
              testIdPrefix="bridge-create"
              workspaceDisabled={!activeWorkspaceId}
              workspaceId={activeWorkspaceId ?? null}
              workspaces={workspaceOptions}
            />
          }
          trailingLabel="Scope"
        />

        <EntityDialogBody className="flex flex-col">
          {secretRecovery ? (
            <>
              <Alert data-testid="bridge-create-secret-recovery" variant="danger">
                <AlertDescription>
                  The bridge was created, but {failedSlotNames.length}{" "}
                  {failedSlotNames.length === 1 ? "credential" : "credentials"} could not be bound.
                  Re-enter {failedSlotNames.join(", ")} and retry — the earlier values were not
                  stored and are never shown again.
                </AlertDescription>
              </Alert>
              <ImmutableIdentity
                hint="Provider and account are fixed for this bridge."
                rows={[
                  { label: "Bridge", mono: true, value: secretRecovery.bridgeId },
                  { label: "Platform", mono: true, value: selectedProvider?.platform ?? "—" },
                  {
                    label: "Extension",
                    mono: true,
                    value: selectedProvider?.extension_name ?? "—",
                  },
                ]}
              />
            </>
          ) : (
            <BridgeCreateSimpleSection
              draft={draft}
              onDraftChange={onDraftChange}
              onSelectProvider={selectProvider}
              providerSelectionDisabled={isPending}
              providers={providers}
              supportsManifest={supportsManifest}
            />
          )}

          <BridgeCreateSecretSlots
            bound={secretRecovery?.bound}
            bridgeId={secretRecovery?.bridgeId ?? null}
            deferredHint={
              supportsManifest
                ? "Slack issues these after the app is installed. Leave them empty now and bind them from the bridge detail."
                : undefined
            }
            failures={recoveryFailures}
            onValueChange={(name, value) =>
              onDraftChange({
                ...draft,
                secretSlotValues: { ...draft.secretSlotValues, [name]: value },
              })
            }
            saving={isPending}
            slots={selectedProvider?.secret_slots}
            values={draft.secretSlotValues}
          />

          {mode === "advanced" && !secretRecovery ? (
            <BridgeCreateAdvancedSection
              draft={draft}
              onDraftChange={onDraftChange}
              provider={selectedProvider}
              providerConfigError={providerConfigError}
            />
          ) : null}
        </EntityDialogBody>

        <EntityDialogFooter
          cancelDisabled={isPending}
          cancelLabel={secretRecovery ? "Open bridge" : "Cancel"}
          cancelTestId="bridge-wizard-cancel"
          hint={
            secretRecovery
              ? "The bridge already exists — retrying only writes the missing credentials."
              : "Provider and account are fixed after creation — create a new bridge to change them."
          }
          isSaving={isPending}
          onCancel={() => handleOpenChange(false)}
          primaryDisabled={secretRecovery ? isPending || !recoveryReady : !canSubmit || isPending}
          primaryLabel={
            secretRecovery
              ? "Retry secret binding"
              : supportsManifest
                ? "Create and continue"
                : "Create bridge"
          }
          primaryTestId="submit-bridge-create"
          onPrimary={onSubmit}
        />
      </DialogContent>
    </Dialog>
  );
}
