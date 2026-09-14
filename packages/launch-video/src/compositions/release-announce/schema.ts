import { z } from "zod";

/**
 * Props are schema-backed so the next release is a Studio edit — change the version and the two
 * feature lines in the sidebar, re-render, done. No code change.
 */
export const releaseAnnounceSchema = z.object({
  version: z.string().min(1),
  features: z.array(z.string().min(1)).min(1).max(2),
});

export type ReleaseAnnounceProps = z.infer<typeof releaseAnnounceSchema>;
