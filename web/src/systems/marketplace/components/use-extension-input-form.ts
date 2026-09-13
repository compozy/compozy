import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";

import { useStoreBinding } from "@/hooks/use-store-binding";
import {
  createExtensionInputDraft,
  prepareExtensionInputs,
  type ExtensionInputDefinitions,
} from "./extension-install-model";

const inputFormLogic = createStoreLogic({
  context: (definitions: ExtensionInputDefinitions) => ({
    draft: createExtensionInputDraft(definitions),
  }),
  on: {
    inputChanged: (context, event: { id: string; value: string | boolean }) => ({
      draft: { ...context.draft, [event.id]: event.value },
    }),
  },
});

export function useExtensionInputForm(definitions: ExtensionInputDefinitions, identity: string) {
  const { store, replace } = useStoreBinding(JSON.stringify([identity, definitions]), () =>
    inputFormLogic.createStore(definitions)
  );
  const draft = useSelector(store, snapshot => snapshot.context.draft);
  return {
    definitions,
    draft,
    ...prepareExtensionInputs(definitions, draft),
    change: (id: string, value: string | boolean) => {
      if (definitions.some(input => input.id === id)) store.trigger.inputChanged({ id, value });
    },
    reset: replace,
  };
}

export type ExtensionInputForm = ReturnType<typeof useExtensionInputForm>;
