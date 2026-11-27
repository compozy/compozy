import { LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";

import type { MergeSessionComputer } from "./merge-sessions";

export interface MergeableAutomationJob {
  readonly id: string;
  readonly name: string;
  readonly updated_at?: string | null;
  readonly created_at?: string | null;
}

export interface ComputerAutomationJobsInput<T extends MergeableAutomationJob> {
  readonly computer: MergeSessionComputer;
  readonly jobs: readonly T[];
}

export interface MergedAutomationJob<T extends MergeableAutomationJob> {
  readonly job: T;
  readonly computerId: ComputerId;
  readonly computerName: string;
  readonly isLocal: boolean;
  readonly offline: boolean;
  readonly platform?: string;
}

/**
 * Merges per-computer automation job lists into one ordered list while preserving
 * each job's owning computer and offline read-only state (US-009.AC-4, IT-046).
 */
export function mergeAutomationJobPages<T extends MergeableAutomationJob>(
  pages: readonly ComputerAutomationJobsInput<T>[]
): MergedAutomationJob<T>[] {
  const merged: MergedAutomationJob<T>[] = [];
  for (const page of pages) {
    const isLocal = page.computer.id === LOCAL_COMPUTER_ID;
    const offline = !isLocal && page.computer.status === "offline";
    for (const job of page.jobs) {
      merged.push({
        job,
        computerId: page.computer.id,
        computerName: page.computer.name,
        isLocal,
        offline,
        platform: page.computer.platform,
      });
    }
  }
  merged.sort((left, right) => {
    const nameOrder = left.job.name.localeCompare(right.job.name);
    if (nameOrder !== 0) return nameOrder;
    return left.computerId.localeCompare(right.computerId);
  });
  return merged;
}
