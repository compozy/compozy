import type React from "react";
import { useState } from "react";

import type { FilterFieldConfig } from "../filter-types";
import { scheduleFilterDomSync, useFilterContext } from "./use-filter-context";

const VALIDATION_RESET_KEYS = new Set([
  "Tab",
  "Escape",
  "Enter",
  "ArrowUp",
  "ArrowDown",
  "ArrowLeft",
  "ArrowRight",
]);

interface UseFilterInputOptions<T = unknown> {
  field?: FilterFieldConfig<T>;
  focusOnMount?: boolean;
  onBlur?: React.FocusEventHandler<HTMLInputElement>;
  onKeyDown?: React.KeyboardEventHandler<HTMLInputElement>;
  pattern?: string;
}

export function useFilterInput<T = unknown>({
  field,
  focusOnMount,
  onBlur,
  onKeyDown,
  pattern: inputPattern,
}: UseFilterInputOptions<T>) {
  const context = useFilterContext();
  const [isValid, setIsValid] = useState(true);
  const [validationMessage, setValidationMessage] = useState("");

  const focusInputOnMount = (node: HTMLInputElement | null) => {
    if (node && focusOnMount) {
      scheduleFilterDomSync(() => node.focus());
    }
  };

  const validateFilterInputOnBlur = (event: React.FocusEvent<HTMLInputElement>) => {
    const value = event.target.value;
    const pattern = field?.pattern || inputPattern;

    if (value && (pattern || field?.validation)) {
      let valid = true;
      let customMessage = "";

      if (field?.validation) {
        const result = field.validation(value);
        if (typeof result === "boolean") {
          valid = result;
        } else {
          valid = result.valid;
          customMessage = result.message || "";
        }
      } else if (pattern) {
        valid = new RegExp(pattern).test(value);
      }

      setIsValid(valid);
      setValidationMessage(valid ? "" : customMessage || context.i18n.validation.invalid);
    } else {
      setIsValid(true);
      setValidationMessage("");
    }

    onBlur?.(event);
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (!isValid && !VALIDATION_RESET_KEYS.has(event.key)) {
      setIsValid(true);
      setValidationMessage("");
    }

    onKeyDown?.(event);
  };

  return {
    context,
    focusInputOnMount,
    handleKeyDown,
    isValid,
    validateFilterInputOnBlur,
    validationMessage,
  };
}
