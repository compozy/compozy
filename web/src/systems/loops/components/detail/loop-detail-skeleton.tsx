import { useNavigate } from "@tanstack/react-router";

import { useTopbarSlot } from "@compozy/ui";

import { LoopDetailSkeletonBody, type LoopDetailSkeletonProps } from "./loop-detail-skeleton-body";
import { loopDetailTopbarIdentity } from "./loop-detail-topbar";

/**
 * The Loop detail page before its definition and settings have answered. The window head already
 * names the Loop and offers the way back, so only its actions arrive late.
 */
export function LoopDetailSkeleton({ name }: LoopDetailSkeletonProps) {
  const navigate = useNavigate();
  useTopbarSlot(loopDetailTopbarIdentity(name, () => void navigate({ to: "/loops" })));
  return <LoopDetailSkeletonBody name={name} />;
}
