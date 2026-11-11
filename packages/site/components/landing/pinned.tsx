"use client";

/** Shows the scroll-pinned stage where there is room and motion is welcome, the static fallback otherwise. */
import type { ReactNode } from "react";

import { usePinning } from "@/components/landing/hooks/use-pinning";

export function Pinned({ stage, fallback }: { stage: ReactNode; fallback: ReactNode }) {
  return usePinning() ? stage : fallback;
}
