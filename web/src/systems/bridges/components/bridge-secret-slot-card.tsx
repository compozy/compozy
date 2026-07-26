import { Trash2 } from "lucide-react";

import {
  Button,
  ConfirmDialog,
  DialogTrigger,
  Eyebrow,
  Field,
  FieldContent,
  FieldDescription,
  FieldTitle,
  Input,
  Pill,
  type PillTone,
} from "@agh/ui";

import { describeBridgeSecretSlot } from "../lib/bridge-formatters";
import type { BridgeProvider, BridgeSecretBinding, BridgeVerifyCheck } from "../types";

interface BridgeSecretSlotCardProps {
  binding?: BridgeSecretBinding;
  checks: readonly BridgeVerifyCheck[];
  inputValue: string;
  isSecretBindingPending: boolean;
  onDelete?: (bindingName: string) => void;
  onDraftChange?: (bindingName: string, value: string) => void;
  onSave?: (bindingName: string) => void;
  slot: NonNullable<BridgeProvider["secret_slots"]>[number];
}

function checkPresentation(status: BridgeVerifyCheck["status"]): {
  label: string;
  tone: PillTone;
} {
  switch (status) {
    case "pass":
      return { label: "PASS", tone: "success" };
    case "warn":
      return { label: "WARN", tone: "warning" };
    case "fail":
      return { label: "FAIL", tone: "danger" };
    case "skipped":
      return { label: "SKIPPED", tone: "neutral" };
  }
}

function SecretCheck({ check, slotName }: { check: BridgeVerifyCheck; slotName: string }) {
  const presentation = checkPresentation(check.status);
  const operatorLabel =
    check.check === "provider.identity"
      ? check.status === "pass"
        ? `${slotName} valid`
        : `${slotName} check`
      : check.check;
  return (
    <li
      className="rounded border border-line bg-canvas px-3 py-2"
      data-testid={`bridge-secret-check-${slotName}-${check.check}`}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="min-w-0">
          <p className="text-small-body font-medium text-fg">{operatorLabel}</p>
          <p className="mt-1 break-all font-mono text-eyebrow text-subtle">{check.check}</p>
        </div>
        <Pill mono tone={presentation.tone}>
          {presentation.label}
        </Pill>
      </div>
      {check.remediation ? (
        <p className="mt-2 text-xs leading-relaxed text-muted">{check.remediation}</p>
      ) : null}
    </li>
  );
}

export function BridgeSecretSlotCard({
  binding,
  checks,
  inputValue,
  isSecretBindingPending,
  onDelete,
  onDraftChange,
  onSave,
  slot,
}: BridgeSecretSlotCardProps) {
  return (
    <article
      className="min-w-0 rounded-md border border-line bg-canvas-soft px-4 py-3"
      data-testid={`bridge-secret-binding-${slot.name}`}
    >
      <div className="flex flex-wrap items-center gap-2">
        <Eyebrow className="text-accent">{slot.name}</Eyebrow>
        <Pill mono tone={slot.required === false ? "neutral" : "warning"}>
          {slot.required === false ? "OPTIONAL" : "REQUIRED"}
        </Pill>
        <Pill mono tone={binding ? "success" : "neutral"}>
          {binding ? "BOUND" : "UNBOUND"}
        </Pill>
      </div>
      <p className="mt-2 text-xs leading-relaxed text-muted">{describeBridgeSecretSlot(slot)}</p>

      {checks.length > 0 ? (
        <ul aria-label={`${slot.name} verification checks`} className="mt-3 space-y-2">
          {checks.map(check => (
            <SecretCheck check={check} key={check.check} slotName={slot.name} />
          ))}
        </ul>
      ) : null}

      <div className="mt-3 grid min-w-0 gap-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
        <Field className="min-w-0">
          <FieldContent className="min-w-0">
            <FieldTitle>Secret value</FieldTitle>
            <FieldDescription>
              AGH stores bridge secret values in the vault for this bridge.
            </FieldDescription>
          </FieldContent>
          <Input
            data-testid={`bridge-secret-env-input-${slot.name}`}
            id={`bridge-secret-env-${slot.name}`}
            onChange={event => onDraftChange?.(slot.name, event.target.value)}
            placeholder="Paste secret value"
            type="password"
            value={inputValue}
          />
          {binding ? (
            <p className="min-w-0 text-xs text-muted">
              Current ref: <span className="break-all font-mono">{binding.secret_ref}</span>
            </p>
          ) : (
            <p className="text-xs text-subtle">No secret binding stored.</p>
          )}
        </Field>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            data-testid={`save-bridge-secret-${slot.name}`}
            disabled={!inputValue.trim() || isSecretBindingPending}
            onClick={() => onSave?.(slot.name)}
            size="sm"
            type="button"
          >
            Save
          </Button>
          <ConfirmDialog
            cancelButtonProps={{
              "data-testid": `cancel-delete-bridge-secret-${slot.name}`,
              disabled: isSecretBindingPending,
            }}
            cancelLabel="Cancel"
            confirmButtonProps={{ "data-testid": `confirm-delete-bridge-secret-${slot.name}` }}
            confirmIcon={Trash2}
            confirmLabel="Delete binding"
            description={`This removes the stored vault binding for ${slot.name}. The provider will not receive this secret until a replacement is saved.`}
            isPending={isSecretBindingPending}
            onConfirm={() => onDelete?.(slot.name)}
            title="Delete secret binding?"
            tone="danger"
          >
            <DialogTrigger
              render={
                <Button
                  data-testid={`delete-bridge-secret-${slot.name}`}
                  disabled={!binding || isSecretBindingPending}
                  size="sm"
                  type="button"
                  variant="outline"
                />
              }
            >
              Delete
            </DialogTrigger>
          </ConfirmDialog>
        </div>
      </div>
    </article>
  );
}
