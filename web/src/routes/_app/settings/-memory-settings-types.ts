import type { Dispatch, SetStateAction } from "react";

import type { SettingsMemorySection } from "@/systems/settings";

export type MemoryConfig = SettingsMemorySection["config"];
export type ValidationSetter = (key: string) => (message: string | null) => void;
export const TEST_PREFIX = "settings-page-memory";

export interface DraftSectionProps {
  draft: MemoryConfig;
  setDraft: Dispatch<SetStateAction<MemoryConfig | null>>;
}

export interface ValidatedSectionProps extends DraftSectionProps {
  validationErrors: Record<string, string | null>;
  setValidationError: ValidationSetter;
}
