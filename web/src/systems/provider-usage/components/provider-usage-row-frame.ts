/**
 * The collapsed row's geometry, shared by the live rows and their loading placeholders so the two
 * cannot drift: the list the rows stack in, the row's disclosure stack, and the one-line trigger
 * (chevron and provider mark, name, plan, cue) with its padding and gaps.
 */
export const PROVIDER_USAGE_LIST_FRAME = "flex flex-col gap-0.5";

export const PROVIDER_USAGE_ROW_FRAME = "flex min-w-0 flex-col rounded-lg";

export const PROVIDER_USAGE_ROW_TRIGGER_FRAME =
  "flex w-full min-w-0 items-center gap-2 rounded-lg px-2 py-1.5";
