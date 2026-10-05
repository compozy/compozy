import type { LoopCatalogEntry } from "../types";

export type LoopAutomationStartKind = "schedule" | "trigger" | "webhook";
export type LoopTargetAvailabilityStatus =
  | "empty"
  | "loading"
  | "compatible"
  | "incompatible"
  | "unavailable";

export interface LoopTargetCatalog {
  error: Error | null;
  fetchNextPage: () => void;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  isLoading: boolean;
  options: LoopCatalogEntry[];
  requiredStartKind: LoopAutomationStartKind;
  selected: LoopCatalogEntry | null;
  selectedName: string;
  status: LoopTargetAvailabilityStatus;
}

interface ProjectLoopTargetCatalogInput {
  error: Error | null;
  fetchNextPage: () => void;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  isLoading: boolean;
  loops: LoopCatalogEntry[];
  requiredStartKind: LoopAutomationStartKind;
  selected: LoopCatalogEntry | null;
  selectedName: string;
}

export function loopDeclaresStartKind(
  loop: LoopCatalogEntry,
  requiredStartKind: LoopAutomationStartKind
): boolean {
  return (loop.start ?? []).some(binding => binding.kind === requiredStartKind);
}

export function projectLoopTargetCatalog({
  error,
  fetchNextPage,
  hasNextPage,
  isFetchingNextPage,
  isLoading,
  loops,
  requiredStartKind,
  selected,
  selectedName,
}: ProjectLoopTargetCatalogInput): LoopTargetCatalog {
  let status: LoopTargetAvailabilityStatus = "empty";
  if (selectedName !== "") {
    if (selected) {
      status = loopDeclaresStartKind(selected, requiredStartKind) ? "compatible" : "incompatible";
    } else {
      status = isLoading ? "loading" : "unavailable";
    }
  }

  return {
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isLoading,
    options: loops.filter(loop => loopDeclaresStartKind(loop, requiredStartKind)),
    requiredStartKind,
    selected,
    selectedName,
    status,
  };
}

export function loopTargetAvailabilityMessage(
  catalog: LoopTargetCatalog,
  mode: "create" | "edit"
): string | null {
  if (catalog.status === "loading") {
    return `Checking whether ${catalog.selectedName} declares the ${catalog.requiredStartKind} start kind.`;
  }
  if (catalog.status === "incompatible") {
    const recovery =
      mode === "edit"
        ? `Update this Loop to allow ${catalog.requiredStartKind} starts before saving.`
        : "Choose a compatible Loop before saving.";
    return `${catalog.selectedName} does not declare the ${catalog.requiredStartKind} start kind. ${recovery}`;
  }
  if (catalog.status === "unavailable") {
    const recovery =
      mode === "edit"
        ? "Restore access to this Loop before saving."
        : "Choose an available Loop before saving.";
    return `${catalog.selectedName} could not be loaded from this workspace. ${recovery}`;
  }
  return null;
}
