import type { PendingWindowManagerCommand } from "./window-manager-store-types";
import type { WindowManagerStoreApi } from "./window-manager-store";

export function beginWindowManagerCommand(
  store: WindowManagerStoreApi,
  command: PendingWindowManagerCommand
): boolean {
  let accepted = false;
  const subscription = store.on("commandAccepted", event => {
    if (event.commandId === command.id) accepted = true;
  });
  try {
    store.trigger.commandBegan({ command });
  } finally {
    subscription.unsubscribe();
  }
  return accepted;
}
