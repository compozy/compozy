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

/** Plain words for each automation start a Loop may allow. */
const START_KIND_COPY: Record<
  LoopAutomationStartKind,
  { by: string; noun: string; fallback: string }
> = {
  schedule: { by: "on a schedule", noun: "scheduled", fallback: "" },
  trigger: { by: "by an event", noun: "event", fallback: ", or start it on a schedule" },
  webhook: { by: "by a link", noun: "link", fallback: ", or start it on a schedule" },
};

export function loopTargetAvailabilityMessage(
  catalog: LoopTargetCatalog,
  mode: "create" | "edit"
): string | null {
  if (catalog.status === "loading") {
    return `Checking whether ${catalog.selectedName} declares the ${catalog.requiredStartKind} start kind.`;
  }
  if (catalog.status === "incompatible") {
    const start = START_KIND_COPY[catalog.requiredStartKind];
    const recovery =
      mode === "edit"
        ? `Update this Loop to allow ${start.noun} starts before saving.`
        : `Choose a Loop that allows ${start.noun} starts${start.fallback}.`;
    return `${catalog.selectedName} can't be started ${start.by}. ${recovery}`;
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
