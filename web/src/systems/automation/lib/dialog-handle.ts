import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";

export type AutomationDialogHandle = ReturnType<typeof DialogPrimitive.createHandle>;

export function createAutomationDialogHandle(): AutomationDialogHandle {
  return DialogPrimitive.createHandle();
}
