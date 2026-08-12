import { createStoreLogic } from "@xstate/store";

const SLASH_PREFIX = /(^|\s)\/([\w-]*)$/u;

interface ComposerState {
  slashFilter: string;
  slashOpen: boolean;
  value: string;
}

type ComposerEvents = {
  changed: { value: string };
  reset: Record<never, never>;
  slashClosed: Record<never, never>;
  slashOpened: Record<never, never>;
  slashSelected: { command: string };
};

function changedState(value: string): ComposerState {
  const match = SLASH_PREFIX.exec(value);
  return {
    slashFilter: match?.[2] ?? "",
    slashOpen: match !== null,
    value,
  };
}

export const composerStateLogic = createStoreLogic<ComposerState, ComposerEvents>({
  context: { slashFilter: "", slashOpen: false, value: "" },
  on: {
    changed: (_context, event) => changedState(event.value),
    reset: () => ({ slashFilter: "", slashOpen: false, value: "" }),
    slashClosed: context => ({ ...context, slashOpen: false }),
    slashOpened: context => ({ ...context, slashFilter: "", slashOpen: true }),
    slashSelected: (context, event) => ({
      slashFilter: "",
      slashOpen: false,
      value: context.value.replace(
        SLASH_PREFIX,
        (_match, leading: string) => `${leading}/${event.command} `
      ),
    }),
  },
});
