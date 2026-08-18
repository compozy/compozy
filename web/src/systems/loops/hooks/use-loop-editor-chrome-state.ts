import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

interface LoopEditorChromeContext {
  paletteOpen: boolean;
  inspectorOpen: boolean;

  dockCollapsed: boolean;
}

export const LOOP_EDITOR_CHROME_STORAGE_KEY = "compozy:loops:editor-chrome:v1";

const DEFAULT_CHROME: LoopEditorChromeContext = {
  paletteOpen: false,
  inspectorOpen: false,
  dockCollapsed: true,
};

function normalizedFlag(value: unknown, fallback: boolean): boolean {
  return typeof value === "boolean" ? value : fallback;
}

function mergeChromePreferences(
  persisted: Partial<LoopEditorChromeContext>,
  current: LoopEditorChromeContext
): LoopEditorChromeContext {
  return {
    paletteOpen: normalizedFlag(persisted.paletteOpen, current.paletteOpen),
    inspectorOpen: normalizedFlag(persisted.inspectorOpen, current.inspectorOpen),
    dockCollapsed: normalizedFlag(persisted.dockCollapsed, current.dockCollapsed),
  };
}

export const loopEditorChromeLogic = createStoreLogic({
  context: (): LoopEditorChromeContext => ({ ...DEFAULT_CHROME }),
  on: {
    paletteToggled: (context): LoopEditorChromeContext => ({
      ...context,
      paletteOpen: !context.paletteOpen,
    }),
    inspectorToggled: (context): LoopEditorChromeContext => ({
      ...context,
      inspectorOpen: !context.inspectorOpen,
    }),
    dockToggled: (context): LoopEditorChromeContext => ({
      ...context,
      dockCollapsed: !context.dockCollapsed,
    }),
    chromeVisibilityChanged: (
      context,
      event: { palette?: boolean; inspector?: boolean; dockCollapsed?: boolean }
    ): LoopEditorChromeContext => ({
      paletteOpen: normalizedFlag(event.palette, context.paletteOpen),
      inspectorOpen: normalizedFlag(event.inspector, context.inspectorOpen),
      dockCollapsed: normalizedFlag(event.dockCollapsed, context.dockCollapsed),
    }),
  },
});

export const loopEditorChromeStore = loopEditorChromeLogic.createStore().with(
  persist({
    name: LOOP_EDITOR_CHROME_STORAGE_KEY,
    merge: mergeChromePreferences,
  })
);

if (typeof window !== "undefined") {
  window.addEventListener("storage", event => {
    if (event.key === LOOP_EDITOR_CHROME_STORAGE_KEY) {
      void rehydrateStore(loopEditorChromeStore);
    }
  });
}

export interface UseLoopEditorChromeStateResult {
  paletteOpen: boolean;
  inspectorOpen: boolean;
  dockCollapsed: boolean;
  togglePalette: () => void;
  toggleInspector: () => void;
  toggleDock: () => void;

  openInspector: () => void;
}

export function useLoopEditorChromeState(): UseLoopEditorChromeStateResult {
  const paletteOpen = useSelector(loopEditorChromeStore, snapshot => snapshot.context.paletteOpen);
  const inspectorOpen = useSelector(
    loopEditorChromeStore,
    snapshot => snapshot.context.inspectorOpen
  );
  const dockCollapsed = useSelector(
    loopEditorChromeStore,
    snapshot => snapshot.context.dockCollapsed
  );

  return {
    paletteOpen,
    inspectorOpen,
    dockCollapsed,
    togglePalette: () => loopEditorChromeStore.trigger.paletteToggled(),
    toggleInspector: () => loopEditorChromeStore.trigger.inspectorToggled(),
    toggleDock: () => loopEditorChromeStore.trigger.dockToggled(),
    openInspector: () => loopEditorChromeStore.trigger.chromeVisibilityChanged({ inspector: true }),
  };
}
