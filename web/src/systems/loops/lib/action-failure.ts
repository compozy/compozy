import { z } from "zod";

const loopActionFailureSchema = z.strictObject({
  kind: z.literal("action_failure"),
  code: z.string().trim().min(1),
  cause: z.string().trim().min(1),
  recovery: z.string().trim().min(1),
});

export type LoopActionFailure = z.infer<typeof loopActionFailureSchema>;

export function parseLoopActionFailure(outputRef: string | undefined): LoopActionFailure | null {
  if (!outputRef) return null;
  try {
    const value: unknown = JSON.parse(outputRef);
    const parsed = loopActionFailureSchema.safeParse(value);
    return parsed.success ? parsed.data : null;
  } catch {
    return null;
  }
}
