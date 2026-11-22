import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { toast } from "@compozy/ui";

import {
  type ExtensionEntry,
  type ExtensionInputsState,
  type ExtensionInputStatus,
  ExtensionsApiError,
  extensionInputsOptions,
  extensionInstallationScope,
  useSetExtensionInputs,
} from "@/systems/extensions";

import type { ExtensionInputDefinitions } from "./extension-install-model";
import { useExtensionInputForm } from "./use-extension-input-form";

export function useExtensionSetupDialog(extension: ExtensionEntry, onClose: () => void) {
  const scope = extensionInstallationScope(extension);
  const state = useQuery(extensionInputsOptions(extension.name, scope));
  const save = useSetExtensionInputs();
  const [remaining, setRemaining] = useState<readonly string[]>([]);
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<string, string>>>({});
  const inputs = state.data?.inputs ?? [];
  const definitions = setupDefinitions(inputs);
  const form = useExtensionInputForm(definitions, `${extension.name}:setup`);
  const formRef = useRef<HTMLFormElement>(null);
  const firstPending = inputs.find(input => input.state !== "set")?.id;
  // Fields arrive after the popup opens: the first input still needing a value takes focus when it
  // appears, including after a save that leaves inputs missing.
  useEffect(() => {
    pendingField(formRef.current, firstPending)?.focus();
  }, [firstPending]);

  const submit = async () => {
    setFieldErrors({});
    try {
      const result = await save.mutateAsync({
        name: extension.name,
        scope,
        inputs: form.inputs,
      });
      form.reset();
      reportSaved(extension.name, result);
      const pending = [...result.missing, ...result.invalid];
      setRemaining(pending);
      if (pending.length === 0) onClose();
    } catch (error) {
      if (error instanceof ExtensionsApiError && error.inputId) {
        setFieldErrors({ [error.inputId]: error.message });
      }
    }
  };

  const saveError =
    save.error && !(save.error instanceof ExtensionsApiError && save.error.inputId)
      ? save.error
      : null;

  return {
    firstPending,
    fieldErrors,
    form,
    formRef,
    inputs,
    remaining,
    save,
    saveError,
    state,
    submit,
  };
}

export function pendingField(
  form: HTMLFormElement | null,
  id: string | undefined
): HTMLElement | null {
  if (!form || id === undefined) return null;
  return form.querySelector<HTMLElement>(`[data-testid="extension-input-${id}"]`);
}

function setupDefinitions(inputs: readonly ExtensionInputStatus[]): ExtensionInputDefinitions {
  return inputs.map(({ binding, id, prompt, type }) => ({
    binding,
    id,
    prompt,
    required: false,
    type,
  }));
}

function reportSaved(name: string, result: ExtensionInputsState) {
  toast.success("Setup saved");
  if (result.reloaded) toast.success(`Restarted ${name}`);
  // MCP-only daemon_env inputs still reach their server through the daemon environment.
  const unreached = result.inputs
    .filter(input => input.state === "daemon_env" && input.targets.includes("subprocess"))
    .map(input => input.id);
  if (unreached.length > 0) {
    toast.info(
      `${unreached.join(", ")} still using the daemon environment; the extension process does not receive it.`
    );
  }
}
