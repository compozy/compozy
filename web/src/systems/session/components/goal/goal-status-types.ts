import type { OperationResponse } from "@/lib/api-contract";

export type SessionGoalSnapshot = NonNullable<OperationResponse<"getSessionGoal", 200>["goal"]>;

export type GoalComposerAffordance =
  | {
      kind: "replace";
      expectedRunId: string;
      objective: string;
    }
  | {
      kind: "draft";
      expandedObjective: string;
    };
