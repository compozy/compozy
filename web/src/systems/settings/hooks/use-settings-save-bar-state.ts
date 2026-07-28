import { useEffect, useRef, useState } from "react";

import {
  deriveSettingsSaveBarState,
  type SettingsSaveBarState,
  type SettingsSaveStateInput,
} from "../lib/save-state";

const SAVED_FLASH_MS = 1400;

type SaveStateOptions = Omit<SettingsSaveStateInput, "showSaved">;

export function useSettingsSaveBarState(options: SaveStateOptions): SettingsSaveBarState {
  const [showSaved, setShowSaved] = useState(false);
  const wasSaving = useRef(options.isSaving);

  useEffect(() => {
    const finishedCleanly =
      wasSaving.current && !options.isSaving && !options.error && !options.isDirty;
    wasSaving.current = options.isSaving;
    if (!finishedCleanly) return;

    setShowSaved(true);
    const timer = window.setTimeout(() => setShowSaved(false), SAVED_FLASH_MS);
    return () => window.clearTimeout(timer);
  }, [options.error, options.isDirty, options.isSaving]);

  return deriveSettingsSaveBarState({ ...options, showSaved });
}
