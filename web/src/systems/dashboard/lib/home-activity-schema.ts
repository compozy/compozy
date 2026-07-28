import { z } from "zod";

/**
 * Guards the load-bearing fields of a `listLogs` event before it enters the
 * activity cache. `looseObject` preserves every other field the daemon sent, so
 * a valid frame keeps its full shape while malformed frames fail closed instead
 * of poisoning the feed with partial rows.
 */
export const homeActivityEventSchema = z.looseObject({
  id: z.string().min(1),
  type: z.string().min(1),
  // The contract is an RFC3339 date-time and the feed parses it with Date; reject
  // anything that is not a real ISO instant so <Time> cannot render "Invalid Date".
  timestamp: z.iso.datetime({ offset: true }),
  outcome: z.string().optional(),
  summary: z.string().optional(),
  agent_name: z.string().optional(),
});
