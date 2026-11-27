import { useQuery } from "@tanstack/react-query";
import { useEffect, useLayoutEffect, useRef, useState } from "react";

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
  // The installed instance owns the draft and the save feedback; another instance starts clean.
  const instance = [extension.name, scope.profileName, scope.workspaceId].join("/");
  const state = useQuery(extensionInputsOptions(extension.name, scope));
  const save = useSetExtensionInputs();
  const [remaining, setRemaining] = useState<readonly string[]>([]);
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<string, string>>>({});
  const [feedbackInstance, setFeedbackInstance] = useState(instance);
  // Every instance switch starts a new generation, so switching away and back still retires a save
  // that began before the switch.
  const [generation, setGeneration] = useState(0);
  // The generation whose save the mutation state reports; an earlier generation's failure stays hidden.
  const [savedGeneration, setSavedGeneration] = useState(generation);
  const currentGeneration = useRef(generation);
  // Layout effect: the guard must see the new generation before a pending save continuation runs.
  useLayoutEffect(() => {
    currentGeneration.current = generation;
  }, [generation]);
  if (feedbackInstance !== instance) {
    setFeedbackInstance(instance);
    setGeneration(next => next + 1);
    setRemaining([]);
    setFieldErrors({});
  }
  const inputs = state.data?.inputs ?? [];
  const definitions = setupDefinitions(inputs);
  const inputForm = useExtensionInputForm(definitions, `${instance}:setup`);
  // Editing a field retires the daemon's rejection of its previous value; other fields keep theirs.
  const form = {
    ...inputForm,
    change: (id: string, value: string | boolean) => {
      inputForm.change(id, value);
      setFieldErrors(errors =>
        errors[id] === undefined ? errors : { ...errors, [id]: undefined }
      );
    },
  };
  const formRef = useRef<HTMLFormElement>(null);
  const firstPending = inputs.find(input => input.state !== "set")?.id;
  // Fields arrive after the popup opens: the first input still needing a value takes focus when it
  // appears, including after a save that leaves inputs missing.
  useEffect(() => {
    pendingField(formRef.current, firstPending)?.focus();
  }, [firstPending]);

  const submit = async () => {
    const submitted = generation;
    setSavedGeneration(submitted);
    setFieldErrors({});
    try {
      const result = await save.mutateAsync({
        name: extension.name,
        scope,
        inputs: form.inputs,
      });
      reportSaved(extension.name, result);
      // A save that resolves after the dialog switched instance must not touch the current form.
      if (currentGeneration.current !== submitted) return;
      form.reset();
      const pending = [...result.missing, ...result.invalid];
      setRemaining(pending);
      if (pending.length === 0) onClose();
    } catch (error) {
      if (currentGeneration.current !== submitted) return;
      if (error instanceof ExtensionsApiError && error.inputId) {
        setFieldErrors({ [error.inputId]: error.message });
      }
    }
  };

  const saveError = savedGeneration === generation ? generalSaveError(save.error) : null;

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

// A failure naming an input shows on its field; any other failure shows on the dialog.
function generalSaveError(error: Error | null): Error | null {
  return error && !(error instanceof ExtensionsApiError && error.inputId) ? error : null;
}

function reportSaved(name: string, result: ExtensionInputsState) {
  toast.success("Setup saved");
  if (result.reloaded) toast.success(`Restarted ${name}`);
  // MCP-only daemon_env inputs still reach their server through the daemon environment.
  const unreached: string[] = [];
  for (const input of result.inputs) {
    if (input.state === "daemon_env" && input.targets.includes("subprocess"))
      unreached.push(input.id);
  }
  if (unreached.length > 0) {
    toast.info(
      `${unreached.join(", ")} still using the daemon environment; the extension process does not receive it.`
    );
  }
}
