import { Link2 } from "lucide-react";

import {
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  EntityModeToolbar,
  Field,
  FieldContent,
  FieldDescription,
  FieldLabel,
  FieldTitle,
  FormSection,
  ImmutableIdentity,
  Input,
  NativeSelect,
  NativeSelectOption,
  RequiredMark,
  type EntityMode,
} from "@agh/ui";

import { parseBridgeProviderConfig } from "../lib/bridge-drafts";
import { describeBridgeDmPolicy } from "../lib/bridge-formatters";
import { isBridgeUpdateDraftDirty } from "../lib/bridge-update-dirty";
import type { BridgeProvider, BridgeUpdateDraft } from "../types";
import {
  BridgeDeliveryTestPanel,
  type BridgeDeliveryTestPanelProps,
} from "./bridge-delivery-test-panel";
import {
  BridgeEditAdvancedSection,
  type BridgeSecretRotation,
} from "./bridge-edit-advanced-section";

interface BridgeEditDialogProps {
  allowProviderDefaultDmPolicy: boolean;
  bridgeName?: string;
  /** Immutable identity reported by the read path. */
  identity?: {
    platform?: string;
    extensionName?: string;
    scope?: string;
    workspaceId?: string | null;
  };
  /** Runtime status line rendered in the toolbar trailing slot. */
  statusLabel?: string;
  draft: BridgeUpdateDraft;
  /** The draft as loaded, so submit can require a real change. */
  pristineDraft: BridgeUpdateDraft;
  isPending: boolean;
  mode?: EntityMode;
  onModeChange: (mode: EntityMode) => void;
  onDraftChange: (draft: BridgeUpdateDraft) => void;
  onOpenChange: (open: boolean) => void;
  onSubmit: () => void;
  open: boolean;
  provider?: BridgeProvider;
  rotation: BridgeEditDialogRotationProps;
  /** A typed rotation is a real change even when no PATCH field moved. */
  hasPendingSecretRotation?: boolean;
  deliveryTest?: Omit<BridgeDeliveryTestPanelProps, "bridgeName">;
}

type BridgeEditDialogRotationProps =
  | {
      kind: "none";
    }
  | {
      kind: "managed";
      rotations: readonly BridgeSecretRotation[];
      onRotationEditingChange: (name: string, editing: boolean) => void;
      onRotationValueChange: (name: string, value: string) => void;
    };

/**
 * Bridge editor on the shared entity-dialog shell.
 *
 * Platform, extension, and scope render as immutable identity because
 * `UpdateBridgeRequest` omits them entirely. The delivery test lives in
 * Advanced (D2) as the single entry point — the standalone dialog is deleted.
 */
export function BridgeEditDialog({
  allowProviderDefaultDmPolicy,
  bridgeName,
  identity,
  statusLabel,
  draft,
  pristineDraft,
  isPending,
  mode = "simple",
  onModeChange,
  onDraftChange,
  onOpenChange,
  onSubmit,
  open,
  provider,
  rotation,
  hasPendingSecretRotation = false,
  deliveryTest,
}: BridgeEditDialogProps) {
  const rotations = rotation.kind === "managed" ? rotation.rotations : [];
  const providerConfigError = parseBridgeProviderConfig(draft.providerConfigText).error;
  // PATCH rejects an empty patch (`HasChanges`), so the primary stays inert
  // until a mutable field actually differs from what was loaded.
  const isDirty = isBridgeUpdateDraftDirty(draft, pristineDraft) || hasPendingSecretRotation;
  const canSubmit = Boolean(draft.displayName.trim()) && !providerConfigError && isDirty;

  const identityRows = identity
    ? [
        ...(identity.platform ? [{ label: "Platform", mono: true, value: identity.platform }] : []),
        ...(identity.extensionName
          ? [{ label: "Extension", mono: true, value: identity.extensionName }]
          : []),
        ...(identity.scope ? [{ label: "Scope", mono: true, value: identity.scope }] : []),
        ...(identity.workspaceId
          ? [{ label: "Workspace", mono: true, value: identity.workspaceId }]
          : []),
      ]
    : [];

  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("md")}`}
        data-testid="bridge-edit-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={
            <>
              Update delivery behavior and rotate credentials.{" "}
              <b className="font-medium text-muted">Identity stays fixed</b> and stored secrets are
              never displayed.
            </>
          }
          eyebrow="Catalog · Bridge"
          icon={Link2}
          onClose={isPending ? undefined : () => onOpenChange(false)}
          title={bridgeName ? `Edit ${bridgeName}` : "Edit bridge"}
        />

        <EntityModeToolbar
          mode={mode}
          onModeChange={onModeChange}
          testIdPrefix="bridge-edit"
          trailing={
            statusLabel ? (
              <span
                className="font-mono text-form-label text-muted"
                data-testid="bridge-edit-status"
              >
                {statusLabel}
              </span>
            ) : null
          }
        />

        <EntityDialogBody className="flex flex-col">
          {identityRows.length > 0 ? (
            <ImmutableIdentity
              hint="Provider and account are fixed for this bridge — create a new bridge to change them."
              rows={identityRows}
            />
          ) : null}

          <FormSection
            data-testid="bridge-edit-section-identity"
            description="How this bridge appears in lists and receipts."
            icon={Link2}
            title="Identity"
          >
            <Field>
              <FieldLabel htmlFor="bridge-edit-display-name-input">
                Display name
                <RequiredMark />
              </FieldLabel>
              <Input
                data-testid="bridge-edit-display-name-input"
                id="bridge-edit-display-name-input"
                onChange={event => onDraftChange({ ...draft, displayName: event.target.value })}
                placeholder="Support operations"
                value={draft.displayName}
              />
            </Field>

            <Field>
              <FieldContent>
                <FieldTitle>DM policy</FieldTitle>
                <FieldDescription>
                  {describeBridgeDmPolicy(draft.dmPolicy === "" ? undefined : draft.dmPolicy)}
                </FieldDescription>
              </FieldContent>
              <NativeSelect
                aria-label="Direct message policy"
                data-testid="bridge-edit-dm-policy-select"
                onChange={event =>
                  onDraftChange({
                    ...draft,
                    dmPolicy: event.target.value as BridgeUpdateDraft["dmPolicy"],
                  })
                }
                value={draft.dmPolicy}
              >
                {allowProviderDefaultDmPolicy ? (
                  <NativeSelectOption value="">Use provider default</NativeSelectOption>
                ) : null}
                <NativeSelectOption value="open">Open</NativeSelectOption>
                <NativeSelectOption value="allowlist">Allowlist</NativeSelectOption>
                <NativeSelectOption value="pairing">Pairing</NativeSelectOption>
              </NativeSelect>
            </Field>
          </FormSection>

          {mode === "advanced" ? (
            <>
              <BridgeEditAdvancedSection
                draft={draft}
                onDraftChange={onDraftChange}
                onRotationEditingChange={
                  rotation.kind === "managed" ? rotation.onRotationEditingChange : undefined
                }
                onRotationValueChange={
                  rotation.kind === "managed" ? rotation.onRotationValueChange : undefined
                }
                provider={provider}
                providerConfigError={providerConfigError}
                rotations={rotations ?? []}
              />
              {deliveryTest ? (
                <BridgeDeliveryTestPanel {...deliveryTest} bridgeName={bridgeName} />
              ) : null}
            </>
          ) : null}
        </EntityDialogBody>

        <EntityDialogFooter
          cancelDisabled={isPending}
          cancelTestId="bridge-edit-cancel"
          hint="Only changed mutable fields are sent."
          isSaving={isPending}
          onCancel={() => onOpenChange(false)}
          onPrimary={onSubmit}
          primaryDisabled={!canSubmit || isPending}
          primaryLabel={isPending ? "Saving…" : "Save changes"}
          primaryTestId="submit-bridge-edit"
        />
      </DialogContent>
    </Dialog>
  );
}
