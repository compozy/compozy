import type { OperationResponse } from "@/lib/api-contract";

export type OnboardingStatusResponse = OperationResponse<"getOnboardingStatus", 200>;
export type OnboardingStatus = OnboardingStatusResponse["onboarding"];

export type FSBrowseResponse = OperationResponse<"browseDirectory", 200>;
export type FSEntry = FSBrowseResponse["entries"][number];

export interface DirectoryBrowseQuery {
  path?: string;
  showHidden?: boolean;
  dirsOnly?: boolean;
}
