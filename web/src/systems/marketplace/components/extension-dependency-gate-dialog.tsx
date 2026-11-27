import type { ExtensionDependencyGate } from "./extension-dependency-model";
import { ExtensionTrustDialog } from "./extension-trust-dialog";

/**
 * The existing trust and gateway dialogs, titled with the dependency they gate. Dismissing either
 * cancels the whole Add before anything is installed.
 */
export function ExtensionDependencyGateDialog({
  gate,
  onAccept,
  onCancel,
}: {
  gate: ExtensionDependencyGate | null;
  onAccept: () => void;
  onCancel: () => void;
}) {
  if (!gate) return null;
  const onOpenChange = (open: boolean) => {
    if (!open) onCancel();
  };
  return (
    <ExtensionTrustDialog
      action="install"
      description={`${gate.requiredBy.join(", ")} needs ${gate.name}, which isn't verified. Installing it runs code with the permissions it declares.`}
      key={`trust:${gate.name}`}
      name={gate.name}
      onConfirm={onAccept}
      onOpenChange={onOpenChange}
      open
    />
  );
}
