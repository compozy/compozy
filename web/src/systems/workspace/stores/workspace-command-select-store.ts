import { createStoreLogic } from "@xstate/store";

interface WorkspaceCommandSelectContext {
  activeCommandValue: string;
  open: boolean;
  pointerSelectionArmed: boolean;
  submenu: {
    focus: "none" | "pending";
    workspaceId: string;
  } | null;
}

type WorkspaceCommandSelectEvents = {
  addWorkspaceSelected: { add: () => void; setOpen?: (open: boolean) => void };
  commandValueChanged: { value: string };
  openChanged: { open: boolean; setOpen?: (open: boolean) => void };
  pointerSelectionArmed: {};
  submenuKeyboardOpened: { workspaceId: string };
  submenuOpenChanged: { open: boolean; workspaceId: string };
  workspaceSelected: {
    gitBacked: boolean;
    select: () => void;
    setOpen?: (open: boolean) => void;
    workspaceId: string;
  };
};

const closedState = {
  open: false,
  pointerSelectionArmed: false,
  submenu: null,
} as const;

export const workspaceCommandSelectLogic = createStoreLogic<
  WorkspaceCommandSelectContext,
  WorkspaceCommandSelectEvents
>({
  context: { activeCommandValue: "", ...closedState },
  on: {
    addWorkspaceSelected: (context, event, enqueue) => {
      enqueue.effect(() => {
        event.add();
        event.setOpen?.(false);
      });
      return { ...context, ...closedState };
    },
    commandValueChanged: (context, event) => ({
      ...context,
      activeCommandValue: event.value,
      pointerSelectionArmed: false,
      submenu: null,
    }),
    openChanged: (context, event, enqueue) => {
      enqueue.effect(() => event.setOpen?.(event.open));
      return event.open ? { ...context, open: true } : { ...context, ...closedState };
    },
    pointerSelectionArmed: context => ({ ...context, pointerSelectionArmed: true }),
    submenuKeyboardOpened: (context, event) => ({
      ...context,
      pointerSelectionArmed: false,
      submenu: { focus: "pending", workspaceId: event.workspaceId },
    }),
    submenuOpenChanged: (context, event) => {
      if (!event.open) return { ...context, submenu: null };
      const focus =
        context.submenu?.workspaceId === event.workspaceId ? context.submenu.focus : "none";
      return {
        ...context,
        submenu: { focus, workspaceId: event.workspaceId },
      };
    },
    workspaceSelected: (context, event, enqueue) => {
      if (event.gitBacked && !context.pointerSelectionArmed) {
        return {
          ...context,
          pointerSelectionArmed: false,
          submenu: { focus: "none", workspaceId: event.workspaceId },
        };
      }
      enqueue.effect(() => {
        event.select();
        event.setOpen?.(false);
      });
      return { ...context, ...closedState };
    },
  },
});
