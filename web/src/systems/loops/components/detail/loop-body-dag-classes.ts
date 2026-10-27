// Shared by the real graph and its skeleton so a step card is the same size either way. In a module
// of their own so the skeleton does not bring the graph's helpers with it.
export const DAG_LANE_CLASS = "flex items-stretch gap-0 overflow-x-auto p-4";
export const DAG_CARD_CLASS =
  "flex w-31 shrink-0 flex-col gap-1 overflow-hidden rounded-md bg-card px-3 py-2.5 shadow-card";
export const DAG_ARROW_CLASS = "flex items-center self-center px-1 text-faint";
